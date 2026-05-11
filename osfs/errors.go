package osfs

import "errors"

// ErrPathEscapesParent represents when an action leads to escaping from the
// dir the filesystem is bound to.
//
// On non-js builds this aligns with [os.Root]'s containment guarantees; the
// upstream error returned by os.Root is not exported, so this sentinel is
// exposed instead. On js/wasm the symbol is defined for API parity but is
// not returned by the in-memory implementation.
var ErrPathEscapesParent = errors.New("path escapes from parent")
