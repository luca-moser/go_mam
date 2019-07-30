package mam

/*
#cgo CFLAGS: -Imam -Iuthash/src -Ientangled
#cgo LDFLAGS: -L. -lmam -lkeccak
#include <mam/api/api.h>

int mam_mss_max_skn(int depth) {
	return MAM_MSS_MAX_SKN(depth);
}

trits_t mam_trits_create(size_t k) {
	MAM_TRITS_DEF(t, k);
	return MAM_TRITS_INIT(t, k);
}
*/
import "C"

// MSSMaxSKN returns the maximum amount of secret keys for the given MSS depth.
func MSSMaxSKN(depth uint) int {
	return int(C.mam_mss_max_skn(C.int(depth)))
}

/*
// creates and inits a C trits_t struct with the given amount of trits
func _c_macro_trits_create(k int) C.trits_t {
	return C.mam_trits_create(C.size_t(k))
}

*/