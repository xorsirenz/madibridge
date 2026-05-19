package matrix

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"

	"github.com/xorsirenz/madibridge/internal/utils"
)

type Client struct {
	*mautrix.Client
}

func New(homeserver, userID, token string) (*Client, error) {
	c, err := mautrix.NewClient(homeserver, id.UserID(userID), token)
	if err != nil {
		return nil, err
	}

	c.Client = &http.Client{
		Transport: utils.RoundTripper{},
	}

	c.UserAgent = "madibridge/0.1"

	return &Client{c}, nil
}

func (c *Client) ResolveMatrixRoom(room string) (string, error) {
	if strings.HasPrefix(room, "!") {
		return room, nil
	}

	if !strings.HasPrefix(room, "#") {
		return "", fmt.Errorf("invalid room id: %s", room)
	}

	resp, err := c.ResolveAlias(context.Background(), id.RoomAlias(room))
	if err != nil {
		return "", fmt.Errorf("failed to resolve alias %s: %w", room, err)
	}

	return resp.RoomID.String(), nil
}
