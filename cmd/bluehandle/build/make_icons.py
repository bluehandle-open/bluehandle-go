"""Draw the app icon and the three tray icons.

Run from anywhere: python -I make_icons.py <output dir>
Needs Pillow. The shapes follow the phone app's logo: a gamepad outline on
a blue rounded square; tray icons add a status dot in the corner.
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
    im = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    d.rounded_rectangle((0, 0, S - 1, S - 1), radius=S * 0.22, fill=BLUE)
    w = S * 0.055
    # gamepad body: viewBox x 2..22, y 7..18 scaled into the middle of the tile
    k = S / 24 * 0.78
    ox, oy = (S - 24 * k) / 2, (S - 24 * k) / 2

    def p(x, y):
        return ox + x * k, oy + y * k

    d.rounded_rectangle((*p(2, 7), *p(22, 18)), radius=5.5 * k, outline=WHITE, width=round(w))
    d.line((*p(7, 10.6), *p(7, 14.4)), fill=WHITE, width=round(w))
    d.line((*p(5.1, 12.5), *p(8.9, 12.5)), fill=WHITE, width=round(w))
    for cx, cy in ((16, 11.5), (18, 13.5)):
        r = 1.15 * k
        x, y = p(cx, cy)
        d.ellipse((x - r, y - r, x + r, y + r), fill=WHITE)
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
