package share

import (
	"testing"

	"github.com/chaitin/panda-wiki/domain"
)

func TestShareNodeOwnsFileKeyAllowsLegacyKnowledgeBasePrefix(t *testing.T) {
	meta := domain.NodeMeta{
		FileKey:    "legacy-kb/source.pdf",
		PreviewKey: "legacy-kb/preview.pdf",
		Attachments: []domain.NodeAttachmentMeta{
			{FileKey: "legacy-kb/attachment.docx", PreviewKey: "legacy-kb/attachment-preview.pdf"},
		},
	}

	for _, key := range []string{
		"legacy-kb/source.pdf",
		"legacy-kb/preview.pdf",
		"legacy-kb/attachment.docx",
		"legacy-kb/attachment-preview.pdf",
	} {
		if !shareNodeOwnsFileKey(meta, key) {
			t.Errorf("shareNodeOwnsFileKey(%q) = false, want true", key)
		}
	}

	if shareNodeOwnsFileKey(meta, "legacy-kb/unrelated.pdf") {
		t.Fatal("shareNodeOwnsFileKey accepted a file key not recorded on the node")
	}
}
