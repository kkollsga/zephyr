//go:build !darwin

package render

// emojiFamily is empty on platforms where we don't ship an explicit color-emoji
// fallback; fontscan resolves whatever the system provides (or tofu).
const emojiFamily = ""
