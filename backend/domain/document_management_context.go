package domain

import "context"

type documentManagementContextKey struct{}
type documentManagementContext struct {
	KBID, UserID string
	Access       *DocumentManagementAccess
}

func WithDocumentManagementAccess(ctx context.Context, kbID, userID string, access *DocumentManagementAccess) context.Context {
	return context.WithValue(ctx, documentManagementContextKey{}, documentManagementContext{KBID: kbID, UserID: userID, Access: access})
}
func DocumentManagementAccessFromContext(ctx context.Context, kbID, userID string) *DocumentManagementAccess {
	value, ok := ctx.Value(documentManagementContextKey{}).(documentManagementContext)
	if !ok || value.KBID != kbID || value.UserID != userID {
		return nil
	}
	return value.Access
}
