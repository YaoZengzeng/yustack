package buffer

// View is a slice of a buffer, with convenience methods
type View []byte

// NewView allocates a new buffer and returns an initialized view that convers
// the whole buffer
func NewView(size int) View {
	return make(View, size)
}

// CapLength irreversibly reduces the length of the visible section of the
// buffer to the value specified
func (v *View) CapLength(length int) {
	// We also set the slice cap because if we don't, one would be able to
	// expand the view back to include the region just excluded. We want to
	// prevent that to avoid potential data leak if we have uninitialized
	// data in excluding region
	*v = (*v)[:length:length]
}

// TrimFront removes the first "count" bytes from the visible section of the
// buffer
func (v *View) TrimFront(count int) {
	*v = (*v)[count:]
}

// VectorisedView is a vectorised version of View using non contigous memory
// It supports all the convenience methods supported by View
type VectorisedView struct {
	views	[]View
	size 	int
}

// NewVectorisedView creates a new vectorised view from an already-allocated slice
// of View and sets its size
func NewVectorisedView(views []View, size int) VectorisedView {
	return VectorisedView{views: views, size: size}
}

// SetSize unsafely sets the size of the VectorisedView
func (vv *VectorisedView) SetSize(size int) {
	vv.size = size
}

// SetViews unsafely sets the views of the VectorisedView
func (vv *VectorisedView) SetViews(views []View) {
	vv.views = views
}

// First returns the first view of the vectorised view
// It panics if the vectorised view is empty
func (vv *VectorisedView) First() View {
	if len(vv.views) == 0 {
		panic("vview is empty")
	}
	return vv.views[0]
}

// TrimFront removes the first "count" bytes of the vectorised view
func (vv *VectorisedView) TrimFront(count int) {
	for count > 0 && len(vv.views) > 0 {
		if count < len(vv.views[0]) {
			vv.size -= count
			vv.views[0].TrimFront(count)
			return
		}
		count -= len(vv.views[0])
		vv.RemoveFirst()
	}
}
// RemoveFirst removes the first view of the vectorised view
func (vv *VectorisedView) RemoveFirst() {
	if len(vv.views) == 0 {
		return
	}
	vv.size -= len(vv.views[0])
	vv.views = vv.views[1:]
}

// ToView returns a single view containing the content of the vectorised view
func (vv *VectorisedView) ToView() View {
	v := make([]byte, vv.size)
	u := v
	for i := range vv.views {
		n := copy(u, vv.views[i])
		u = u[n:]
	}
	return v
}

// Size returns the size in bytes of the entire content stored in the vectorised view
func (vv *VectorisedView) Size() int {
	return vv.size
}
