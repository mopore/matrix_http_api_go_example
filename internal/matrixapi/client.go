package matrixapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type Option func(*Client)

type Client struct {
	Homeserver  string
	AccessToken string
	RoomID      string
	HumanUserID string
	BotUserID   string

	httpClient *http.Client
	seenMu     sync.Mutex
	seen       map[string]struct{}
}

func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: 45 * time.Second},
		seen:       make(map[string]struct{}),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func WithHomeserver(url string) Option {
	return func(c *Client) { c.Homeserver = url }
}

func WithAccessToken(token string) Option {
	return func(c *Client) { c.AccessToken = token }
}

func WithRoomID(id string) Option {
	return func(c *Client) { c.RoomID = id }
}

func WithHumanUserID(id string) Option {
	return func(c *Client) { c.HumanUserID = id }
}

func (c *Client) apiRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.Homeserver+path, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d %s", resp.StatusCode, resp.Status)
	}

	return resp, nil
}

func (c *Client) Whoami(ctx context.Context) error {
	resp, err := c.apiRequest(ctx, http.MethodGet, "/_matrix/client/v3/account/whoami", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var data WhoamiResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}
	c.BotUserID = data.UserID
	return nil
}

func (c *Client) SendMessage(ctx context.Context, body string) error {
	txnID := fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().Unix())
	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/send/m.room.message/%s", url.PathEscape(c.RoomID), url.PathEscape(txnID))

	payload := map[string]string{
		"msgtype": "m.text",
		"body":    body,
	}

	resp, err := c.apiRequest(ctx, http.MethodPut, path, payload)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) joinRoom(ctx context.Context, roomID string) error {
	path := fmt.Sprintf("/_matrix/client/v3/join/%s", url.PathEscape(roomID))
	resp, err := c.apiRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) Sync(ctx context.Context) iter.Seq2[*MatrixEvent, error] {
	return func(yield func(*MatrixEvent, error) bool) {
		since := ""

		// Bootstrap initial token
		boot, err := c.syncOnce(ctx, "", 0)
		if err != nil {
			yield(nil, err)
			return
		}
		since = boot.NextBatch

		for {
			select {
			case <-ctx.Done():
				return
			default:
				resp, err := c.syncOnce(ctx, since, 30000)
				if err != nil {
					if !yield(nil, err) {
						return
					}
					time.Sleep(2 * time.Second)
					continue
				}

				since = resp.NextBatch

				// Auto-join
				for rid := range resp.Rooms.Invite {
					_ = c.joinRoom(ctx, rid)
				}

				room, ok := resp.Rooms.Join[c.RoomID]
				if !ok {
					continue
				}

				for _, ev := range room.Timeline.Events {
					if ev.EventID == "" || ev.Type != "m.room.message" || ev.Content.MsgType != "m.text" {
						continue
					}

					c.seenMu.Lock()
					if _, ok := c.seen[ev.EventID]; ok {
						c.seenMu.Unlock()
						continue
					}
					c.seen[ev.EventID] = struct{}{}
					if len(c.seen) > 1000 {
						// Simple eviction
						for k := range c.seen {
							delete(c.seen, k)
							if len(c.seen) <= 1000 {
								break
							}
						}
					}
					c.seenMu.Unlock()

					if ev.Sender == c.BotUserID || ev.Sender != c.HumanUserID {
						continue
					}

					if !yield(&ev, nil) {
						return
					}
				}
			}
		}
	}
}

func (c *Client) syncOnce(ctx context.Context, since string, timeout int) (*SyncResponse, error) {
	filter := `{"room":{"timeline":{"types":["m.room.message"],"limit":20}}}`
	path := fmt.Sprintf("/_matrix/client/v3/sync?timeout=%d&set_presence=offline&filter=%s", timeout, url.QueryEscape(filter))
	if since != "" {
		path += "&since=" + url.QueryEscape(since)
	}

	resp, err := c.apiRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data SyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return &data, nil
}
