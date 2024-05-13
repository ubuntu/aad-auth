package cache

import (
	"context"
)

func (c *Cache) GenerateUIDForUser(ctx context.Context, username string, minUID, maxUID uint32) (uint32, error) {
	return c.generateUIDForUser(ctx, username, minUID, maxUID)
}
