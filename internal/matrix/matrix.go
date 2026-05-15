package matrix

import (
	"net/http"

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
