package wintray

import "image"

// Fit the system keyboard into the bottom of the streamed monitor without
// stretching its current aspect ratio or exceeding the monitor bounds.
func systemKeyboardBounds(screen image.Rectangle, original image.Rectangle) image.Rectangle {
	if screen.Empty() || original.Empty() {
		return image.Rectangle{}
	}
	width := screen.Dx()
	if width > 1100 {
		width = 1100
	}
	height := width * original.Dy() / original.Dx()
	if height > screen.Dy()/2 {
		height = screen.Dy() / 2
		width = height * original.Dx() / original.Dy()
	}
	if width < 1 || height < 1 {
		return image.Rectangle{}
	}
	x := screen.Min.X + (screen.Dx()-width)/2
	return image.Rect(x, screen.Max.Y-height, x+width, screen.Max.Y)
}
