package engine

// WriteGIFForTest encodes an animation through the same writer the tool uses, so
// tests build their fixtures without duplicating the container logic. It lives in
// a _test.go file so it is never compiled into the shipped binary.
func WriteGIFForTest(a *Animation, target string) error {
	return writeGIF(a, target, 90)
}
