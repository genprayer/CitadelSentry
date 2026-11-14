package logger

import (
	"context"
	"log/slog"
	"strings"
)

const maxLogAttributeDepth = 64

// scopedHandler keeps immutable context while sharing the file/rotation owner.
type scopedHandler struct {
	base   *fileHandler
	attrs  []slog.Attr
	groups []string
}

func (h *scopedHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.base.Enabled(ctx, level)
}

func (h *scopedHandler) Handle(ctx context.Context, original slog.Record) error {
	record := slog.NewRecord(original.Time, original.Level, original.Message, original.PC)
	record.AddAttrs(h.attrs...)
	prefix := strings.Join(h.groups, ".")
	original.Attrs(func(attr slog.Attr) bool {
		record.AddAttrs(qualifiedLogAttrs(attr, prefix, 0)...)
		return true
	})
	return h.base.Handle(ctx, record)
}

func (h *scopedHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	cloned := &scopedHandler{base: h.base, groups: append([]string(nil), h.groups...), attrs: append([]slog.Attr(nil), h.attrs...)}
	prefix := strings.Join(h.groups, ".")
	for _, attr := range attrs {
		cloned.attrs = append(cloned.attrs, qualifiedLogAttrs(attr, prefix, 0)...)
	}
	return cloned
}

func (h *scopedHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	groups := append(append([]string(nil), h.groups...), name)
	return &scopedHandler{base: h.base, attrs: h.attrs, groups: groups}
}

func qualifiedLogAttrs(attr slog.Attr, prefix string, depth int) []slog.Attr {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return nil
	}
	key := attr.Key
	if prefix != "" {
		key = prefix
		if attr.Key != "" {
			key += "." + attr.Key
		}
	}
	if attr.Value.Kind() != slog.KindGroup {
		return []slog.Attr{{Key: key, Value: attr.Value}}
	}
	if depth >= maxLogAttributeDepth {
		return []slog.Attr{slog.String(key, "log attribute nesting exceeds the supported limit")}
	}
	var attrs []slog.Attr
	for _, child := range attr.Value.Group() {
		attrs = append(attrs, qualifiedLogAttrs(child, key, depth+1)...)
	}
	return attrs
}
