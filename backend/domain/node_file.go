package domain

// NodeOwnsFileKey deliberately compares exact stored keys. A migrated object may
// retain an old KB prefix, but an unrelated object must never be substituted.
func NodeOwnsFileKey(meta NodeMeta, key string) bool {
	if key == "" {
		return false
	}
	if meta.FileKey == key || meta.PreviewKey == key {
		return true
	}
	for _, a := range meta.Attachments {
		if a.FileKey == key || a.PreviewKey == key {
			return true
		}
	}
	return false
}

func NodeFileName(meta NodeMeta, key string) string {
	if meta.FileKey == key {
		return meta.FileName
	}
	if meta.PreviewKey == key {
		return meta.FileName + ".preview.pdf"
	}
	for _, a := range meta.Attachments {
		if a.FileKey == key {
			return a.FileName
		}
		if a.PreviewKey == key {
			return a.FileName + ".preview.pdf"
		}
	}
	return ""
}
