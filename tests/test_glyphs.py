from __future__ import annotations

import unittest

from pathlib import Path

from app.codex_dashboard.glyphs import GLYPH_KINDS, render_cta, render_glyph

FONT_PATH = Path(__file__).resolve().parents[1] / "app" / "codex_dashboard" / "assets" / "fonts" / "SpaceGrotesk[wght].ttf"


class GlyphRenderTests(unittest.TestCase):
    def test_all_kinds_render_to_a_nonblank_rgba_image_of_requested_size(self) -> None:
        for kind in GLYPH_KINDS:
            image = render_glyph(kind, "#00e5ff", 18)
            self.assertEqual(image.size, (18, 18), kind)
            self.assertEqual(image.mode, "RGBA", kind)
            # The downscaled glyph must have drawn (opaque) pixels — not a blank canvas.
            self.assertGreater(image.getchannel("A").getextrema()[1], 0, kind)

    def test_color_is_honored(self) -> None:
        image = render_glyph("close", "#ff0000", 24).convert("RGBA")
        # The two diagonals of the close "X" cross at the center, so the center pixel is
        # drawn — and it must carry the requested hue (red dominant, opaque).
        r, g, b, a = image.getpixel((12, 12))
        self.assertGreater(a, 100)
        self.assertGreaterEqual(r, g)
        self.assertGreaterEqual(r, b)

    def test_unknown_kind_raises(self) -> None:
        with self.assertRaises(ValueError):
            render_glyph("definitely-not-a-glyph", "#ffffff", 18)

    def test_cta_renders_a_gradient_button_image(self) -> None:
        # BIND_TASK is baked as a gradient image with text + a check glyph.
        image = render_cta("BIND_TASK", FONT_PATH).convert("RGBA")
        self.assertGreater(image.width, image.height)  # a wide button
        # The vertical gradient means the top row is paler (closer to #c3f5ff) than the
        # bottom row (closer to #00e5ff): the top has more red than the bottom.
        top = image.getpixel((image.width // 2, 1))
        bottom = image.getpixel((image.width // 2, image.height - 2))
        self.assertGreater(top[0], bottom[0])


if __name__ == "__main__":
    unittest.main()
