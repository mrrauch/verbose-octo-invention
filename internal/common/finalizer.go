package common

const FinalizerName = "openstack.k8s.io/cleanup"

// objectWithFinalizers is any object that has Get/SetFinalizers.
type objectWithFinalizers interface {
	GetFinalizers() []string
	SetFinalizers([]string)
}

// HasFinalizer returns true if the object has the given finalizer.
func HasFinalizer(obj objectWithFinalizers, finalizer string) bool {
	for _, f := range obj.GetFinalizers() {
		if f == finalizer {
			return true
		}
	}
	return false
}

// AddFinalizer adds the finalizer if not already present.
func AddFinalizer(obj objectWithFinalizers, finalizer string) {
	if !HasFinalizer(obj, finalizer) {
		obj.SetFinalizers(append(obj.GetFinalizers(), finalizer))
	}
}

// RemoveFinalizer removes the finalizer if present.
func RemoveFinalizer(obj objectWithFinalizers, finalizer string) {
	var result []string
	for _, f := range obj.GetFinalizers() {
		if f != finalizer {
			result = append(result, f)
		}
	}
	obj.SetFinalizers(result)
}
