package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitRecipients(t *testing.T) {
	t.Run("clean items pass through trimmed and non-empty", func(t *testing.T) {
		items, err := SplitRecipients(" a@x.com , b@x.com ,,")
		require.NoError(t, err)
		require.Equal(t, []string{"a@x.com", "b@x.com"}, items)
	})

	t.Run("empty value yields no recipients", func(t *testing.T) {
		items, err := SplitRecipients("")
		require.NoError(t, err)
		require.Nil(t, items)
	})

	t.Run("unicode passes unchanged", func(t *testing.T) {
		items, err := SplitRecipients("olá@example.com,Björn@example.com")
		require.NoError(t, err)
		require.Equal(t, []string{"olá@example.com", "Björn@example.com"}, items)
	})

	t.Run("interior CRLF in an item is rejected, naming the item", func(t *testing.T) {
		items, err := SplitRecipients("a@x.com,b\r\nc: evil")
		require.Nil(t, items)
		require.ErrorContains(t, err, `recipient "b\r\nc: evil" contains control characters`)
	})

	t.Run("tab in an item is rejected", func(t *testing.T) {
		_, err := SplitRecipients("a@x.com,b\t@evil.example")
		require.ErrorContains(t, err, "contains control characters")
	})
}
