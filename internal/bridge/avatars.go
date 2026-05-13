package bridge

import (
	"context"
	"fmt"
	"strings"

	"maunium.net/go/mautrix/id"
)

func (b *Bridge) getAvatarURL(ctx context.Context, user id.UserID) string {
	b.cacheLock.RLock()

	if url, ok := b.avatarCache[user]; ok {
		b.cacheLock.RUnlock()
		return url
	}

	b.cacheLock.RUnlock()

	profile, err := b.matrix.Client.GetProfile(ctx, user)
	if err != nil || profile.AvatarURL.String() == "" {
		return ""
	}

	avatarURL := b.mxcToHTTP(profile.AvatarURL.String())

	b.cacheLock.Lock()
	b.avatarCache[user] = avatarURL
	b.cacheLock.Unlock()

	return avatarURL
}

func (b *Bridge) mxcToHTTP(mxc string) string {
	withoutPrefix := strings.TrimPrefix(mxc, "mxc://")
	parts := strings.SplitN(withoutPrefix, "/", 2)

	if len(parts) != 2 {
		return ""
	}

	return fmt.Sprintf("%s/_matrix/media/v3/download/%s/%s", b.cfg.Matrix.Homeserver, parts[0], parts[1])
}
