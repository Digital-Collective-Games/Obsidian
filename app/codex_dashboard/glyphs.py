"""Crisp vector glyphs for the WORKTREES surface, rendered with Pillow.

The earlier Tk-Canvas line art looked "8-bit" because Tk strokes 1px aliased lines at
icon size. Here each glyph is drawn at 4x size and downscaled with LANCZOS, so the edges
are anti-aliased ("4x resolution") — matching the Material-symbol weight of the Stitch
mockup rather than reading as cheap pixel art. Pure function returning a PIL image; the UI
layer wraps it in an ImageTk.PhotoImage and caches it.
"""
from __future__ import annotations

SUPERSAMPLE = 4


def _hex_to_rgba(color: str) -> tuple[int, int, int, int]:
    value = str(color).lstrip("#")
    if len(value) == 3:
        value = "".join(ch * 2 for ch in value)
    r, g, b = (int(value[i : i + 2], 16) for i in (0, 2, 4))
    return (r, g, b, 255)


# Each glyph is defined in a normalized 0..1 box and scaled to the supersampled canvas, so
# the same drawing renders crisply at any size. `kind` keys match the worktree/assign UI.
def render_glyph(kind: str, color: str, size: int, supersample: int = SUPERSAMPLE):
    """Return an RGBA PIL.Image of `kind` drawn in `color` at `size`x`size` px (anti-aliased
    via supersampling). Raises ImportError if Pillow is unavailable (callers fall back)."""
    from PIL import Image, ImageDraw

    size = max(1, int(size))
    rgba = _hex_to_rgba(color)
    work = size * supersample
    image = Image.new("RGBA", (work, work), (0, 0, 0, 0))
    draw = ImageDraw.Draw(image)
    stroke = max(2, round(work * 0.085))

    def pt(x: float, y: float) -> tuple[float, float]:
        return (x * work, y * work)

    def line(points: list[tuple[float, float]], width: int = stroke) -> None:
        draw.line([pt(x, y) for x, y in points], fill=rgba, width=width, joint="curve")

    def rect(x0: float, y0: float, x1: float, y1: float, width: int = stroke) -> None:
        draw.rectangle([pt(x0, y0), pt(x1, y1)], outline=rgba, width=width)

    def circle(x0: float, y0: float, x1: float, y1: float, width: int = stroke) -> None:
        draw.ellipse([pt(x0, y0), pt(x1, y1)], outline=rgba, width=width)

    def disc(x0: float, y0: float, x1: float, y1: float) -> None:
        draw.ellipse([pt(x0, y0), pt(x1, y1)], fill=rgba)

    if kind == "copy":
        # content_copy: a front sheet with a back sheet peeking behind it.
        rect(0.36, 0.34, 0.84, 0.82)
        rect(0.16, 0.16, 0.64, 0.64)
    elif kind == "details":
        # info: "i" inside a circle.
        circle(0.1, 0.1, 0.9, 0.9)
        disc(0.44, 0.27, 0.56, 0.39)
        line([(0.5, 0.46), (0.5, 0.74)])
    elif kind == "destroy":
        # trash can: lid, handle, tapered body, ribs.
        line([(0.16, 0.28), (0.84, 0.28)])
        line([(0.4, 0.16), (0.4, 0.28)])
        line([(0.6, 0.16), (0.6, 0.28)])
        line([(0.4, 0.16), (0.6, 0.16)])
        line([(0.27, 0.3), (0.33, 0.84)])
        line([(0.73, 0.3), (0.67, 0.84)])
        line([(0.33, 0.84), (0.67, 0.84)])
        line([(0.43, 0.42), (0.43, 0.74)])
        line([(0.57, 0.42), (0.57, 0.74)])
    elif kind == "close":
        line([(0.24, 0.24), (0.76, 0.76)])
        line([(0.76, 0.24), (0.24, 0.76)])
    elif kind == "filter":
        # filter_alt: a solid funnel (filled, no trailing droplet).
        draw.polygon(
            [pt(*p) for p in ((0.18, 0.26), (0.82, 0.26), (0.56, 0.52), (0.56, 0.8), (0.44, 0.8), (0.44, 0.52))],
            fill=rgba,
        )
    elif kind == "sort":
        line([(0.2, 0.3), (0.8, 0.3)])
        line([(0.2, 0.5), (0.62, 0.5)])
        line([(0.2, 0.7), (0.46, 0.7)])
    elif kind == "assign":
        # assignment_add: a clipboard (clip + content lines) with a circular plus badge at
        # the lower-right corner.
        rect(0.16, 0.24, 0.62, 0.86)
        line([(0.3, 0.24), (0.3, 0.17), (0.48, 0.17), (0.48, 0.24)])
        line([(0.26, 0.44), (0.52, 0.44)])
        line([(0.26, 0.56), (0.46, 0.56)])
        circle(0.56, 0.56, 0.92, 0.92)
        line([(0.74, 0.62), (0.74, 0.86)])
        line([(0.62, 0.74), (0.86, 0.74)])
    elif kind == "check":
        # check_circle.
        circle(0.1, 0.1, 0.9, 0.9)
        line([(0.3, 0.52), (0.45, 0.68), (0.72, 0.34)])
    elif kind == "radio_on":
        # Material checked form-radio: a solid filled disc with a small light center dot.
        disc(0.12, 0.12, 0.88, 0.88)
        draw.ellipse([pt(0.4, 0.4), pt(0.6, 0.6)], fill=(255, 255, 255, 255))
    elif kind == "radio_off":
        circle(0.14, 0.14, 0.86, 0.86)
    elif kind == "plus":
        line([(0.5, 0.18), (0.5, 0.82)])
        line([(0.18, 0.5), (0.82, 0.5)])
    elif kind == "eject":
        # an eject symbol: a triangle pointing up over a horizontal bar.
        draw.polygon([pt(0.5, 0.2), pt(0.82, 0.56), pt(0.18, 0.56)], fill=rgba)
        draw.rectangle([pt(0.18, 0.66), pt(0.82, 0.78)], fill=rgba)
    elif kind == "sync":
        # two circular arrows (the refresh/sync glyph): a top arc + a bottom arc with a gap
        # on each side, each ending in a tangential arrowhead chevron.
        box = [pt(0.2, 0.2), pt(0.8, 0.8)]
        draw.arc(box, 200, 340, fill=rgba, width=stroke)
        draw.arc(box, 20, 160, fill=rgba, width=stroke)
        line([(0.70, 0.41), (0.82, 0.41), (0.79, 0.52)])
        line([(0.30, 0.59), (0.18, 0.59), (0.21, 0.48)])
    else:
        raise ValueError(f"unknown glyph kind: {kind!r}")

    return image.resize((size, size), Image.LANCZOS)


def render_cta(
    text: str,
    font_path: str,
    *,
    font_px: int = 15,
    side_pad: int = 24,
    top_pad: int = 9,
    gap: int = 8,
    icon_size: int = 16,
    top_color: str = "#c3f5ff",
    bottom_color: str = "#00e5ff",
    fg: str = "#00363d",
    icon: str | None = "check",
    icon_side: str = "right",
    supersample: int = 2,
):
    """Render a CTA button as one image: a vertical gradient fill (the mockup's terminal
    glow), heavy Space Grotesk text, and an optional leading/trailing glyph. Used for every
    worktree action button so they share one size/font/treatment; colors vary by role.
    Raises if Pillow/the font is unavailable so the caller can fall back to a flat button."""
    from PIL import Image, ImageDraw, ImageFont

    ss = supersample
    font = ImageFont.truetype(str(font_path), font_px * ss)
    try:
        font.set_variation_by_axes([900])  # font-black weight on the variable font
    except Exception:
        pass
    measure = ImageDraw.Draw(Image.new("RGBA", (1, 1)))
    bbox = measure.textbbox((0, 0), text, font=font)
    text_w, text_h = bbox[2] - bbox[0], bbox[3] - bbox[1]
    has_icon = icon is not None
    icon_px = icon_size * ss if has_icon else 0
    content_h = max(text_h, icon_px) + 2 * top_pad * ss
    content_w = 2 * side_pad * ss + text_w + ((gap * ss + icon_px) if has_icon else 0)

    image = Image.new("RGBA", (content_w, content_h), (0, 0, 0, 0))
    draw = ImageDraw.Draw(image)
    top_rgb, bottom_rgb = _hex_to_rgba(top_color), _hex_to_rgba(bottom_color)
    for y in range(content_h):
        t = y / max(1, content_h - 1)
        row = tuple(round(top_rgb[i] + (bottom_rgb[i] - top_rgb[i]) * t) for i in range(3)) + (255,)
        draw.line([(0, y), (content_w, y)], fill=row)

    if has_icon and icon_side == "left":
        icon_x = side_pad * ss
        text_x = side_pad * ss + icon_px + gap * ss - bbox[0]
    else:
        text_x = side_pad * ss - bbox[0]
        icon_x = side_pad * ss + text_w + gap * ss
    draw.text((text_x, (content_h - text_h) // 2 - bbox[1]), text, font=font, fill=_hex_to_rgba(fg))
    if has_icon:
        glyph = render_glyph(icon, fg, icon_px, supersample=1)
        image.alpha_composite(glyph, (icon_x, (content_h - icon_px) // 2))
    return image.resize((content_w // ss, content_h // ss), Image.LANCZOS)


GLYPH_KINDS = (
    "copy",
    "details",
    "destroy",
    "close",
    "filter",
    "sort",
    "assign",
    "check",
    "radio_on",
    "radio_off",
    "plus",
    "eject",
    "sync",
)
