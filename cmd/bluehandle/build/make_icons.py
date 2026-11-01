"""Draw the app icon and the three tray icons.

Run from anywhere: python -I make_icons.py <output dir>
Needs Pillow. The shapes follow the phone app's logo: a D-pad with signal
arcs on a flat blue rounded square; tray icons add a status dot in the corner.
"""
import math
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
    """The phone app's launcher icon: a white D-pad with an orange centre and
    two signal arcs in its top-right corner. Geometry matches the phone repo's
    design/logo.py (108-unit canvas, scaled 0.82 and offset by (11.36, 8.08))."""
    im = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    d.rounded_rectangle((0, 0, S - 1, S - 1), radius=S * 0.22, fill=BLUE)
    k = S / 72  # the phone icon's visible 72 units of its 108-unit canvas

    def p(x, y):
        return (x * 0.82 + 11.36 - 18) * k, (y * 0.82 + 8.08 - 18) * k

    def r(v):
        return v * 0.82 * k

    for x0, y0, x1, y1 in ((41, 32, 59, 84), (24, 49, 76, 67)):
        d.rounded_rectangle((*p(x0, y0), *p(x1, y1)), radius=r(3), fill=WHITE)
    cx, cy = p(50, 58)
    d.ellipse((cx - r(5.5), cy - r(5.5), cx + r(5.5), cy + r(5.5)), fill=ORANGE)
    ax, ay = p(62, 46)
    w = r(5.5)
    for radius in (9, 17):
        rad = r(radius)
        d.arc((ax - rad - w / 2, ay - rad - w / 2, ax + rad + w / 2, ay + rad + w / 2), 280, 350, fill=WHITE, width=round(w))
        for deg in (280, 350):  # round caps
            ex, ey = ax + rad * math.cos(math.radians(deg)), ay + rad * math.sin(math.radians(deg))
            d.ellipse((ex - w / 2, ey - w / 2, ex + w / 2, ey + w / 2), fill=WHITE)
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
