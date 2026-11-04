"""Draw the app icon and the three tray icons.

Run from anywhere: python -I make_icons.py <output dir>
Needs Pillow. The shapes follow the phone app's logo: an FC-style capsule
gamepad in white strokes on a flat blue rounded square; tray icons add a status dot in the corner.
"""
import sys
from pathlib import Path

from PIL import Image, ImageDraw

BLUE = (47, 111, 235, 255)
WHITE = (255, 255, 255, 255)
GREEN = (63, 178, 127, 255)
ORANGE = (245, 165, 36, 255)
RING = (20, 24, 34, 255)
S = 512  # draw large, then downsample


def logo() -> Image.Image:
    """The phone app's launcher icon: an FC-style capsule gamepad in white
    strokes, a cross on the left and two buttons on the right (the right one
    orange). Geometry matches the phone repo's design/logo.py (108-unit canvas,
    of which the visible 18..90 maps onto this square)."""
    im = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    d.rounded_rectangle((0, 0, S - 1, S - 1), radius=S * 0.22, fill=BLUE)
    k = S / 72

    z = 0.8  # shrink about the centre for margin, as design/logo.py RASTER_SCALE

    def p(x, y):
        return (54 + (x - 54) * z - 18) * k, (54 + (y - 54) * z - 18) * k

    w = 5 * k * z  # stroke width
    (x0, y0), (x1, y1) = p(22, 39), p(86, 69)  # capsule stroke centre line
    d.rounded_rectangle((x0 - w / 2, y0 - w / 2, x1 + w / 2, y1 + w / 2),
                        radius=(y1 - y0 + w) / 2, outline=WHITE, width=round(w))
    for a, b in (((31.5, 54), (44.5, 54)), ((38, 47.5), (38, 60.5))):  # cross
        d.line((*p(*a), *p(*b)), fill=WHITE, width=round(w))
        for e in (a, b):  # round caps
            ex, ey = p(*e)
            d.ellipse((ex - w / 2, ey - w / 2, ex + w / 2, ey + w / 2), fill=WHITE)
    r = 3.8 * k * z
    for (x, y), colour in (((64, 54), WHITE), ((74, 54), ORANGE)):
        cx, cy = p(x, y)
        d.ellipse((cx - r, cy - r, cx + r, cy + r), fill=colour)
    return im


def with_dot(base: Image.Image, colour) -> Image.Image:
    im = base.copy()
    d = ImageDraw.Draw(im)
    r = S * 0.2
    cx = cy = S - r - S * 0.01
    ring = S * 0.06
    d.ellipse((cx - r - ring, cy - r - ring, cx + r + ring, cy + r + ring), fill=RING)
    d.ellipse((cx - r, cy - r, cx + r, cy + r), fill=colour)
    return im


def main(out: Path) -> None:
    out.mkdir(parents=True, exist_ok=True)
    base = logo()
    base.resize((256, 256), Image.LANCZOS).save(out / "appicon.png")
    base.save(out / "icon.ico", sizes=[(16, 16), (24, 24), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)])
    for name, im in (("tray_idle", base), ("tray_connected", with_dot(base, GREEN)), ("tray_paused", with_dot(base, ORANGE))):
        im.resize((32, 32), Image.LANCZOS).save(out / f"{name}.png")


if __name__ == "__main__":
    main(Path(sys.argv[1]))
