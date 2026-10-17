package gameengine

// r157 — validity for the derived ProducedColorsMask cache.
//
// The mask is computed from OracleTextLower + TypeLineLower. Both of
// those sources are replaced wholesale when a double-faced card
// transforms, so a bare "have we computed this" flag is not enough:
// Search for Azcanta transforms INTO Azcanta, the Sunken Ruin, a land
// that taps for {U}, and the mask computed on the front-face
// enchantment is 0. These two helpers are exported because
// internal/hat carries its own mirror of the same computation
// (landProducesColorsMask) and must agree with the engine's.

// ProducedColorsValid reports whether the cached ProducedColorsMask was
// computed from the card's CURRENT AST and type line.
func (c *Card) ProducedColorsValid() bool {
	if c == nil || !c.ProducedColorsReady {
		return false
	}
	return c.producedColorsAST == c.AST && c.producedColorsTL == c.TypeLine
}

// MarkProducedColors stores a freshly computed mask together with the
// sources it was derived from.
func (c *Card) MarkProducedColors(mask uint8) {
	if c == nil {
		return
	}
	c.ProducedColorsMask = mask
	c.ProducedColorsReady = true
	c.producedColorsAST = c.AST
	c.producedColorsTL = c.TypeLine
}
