"""Draw the app icon and the three tray icons.

Run from anywhere: python -I make_icons.py <output dir>
Needs Pillow. The shapes follow the phone app's logo: a D-pad on a blue
rounded square; tray icons add a status dot in the corner.
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
    """The phone app's launcher icon: a white D-pad with an orange centre key."""
    im = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    d.rounded_rectangle((0, 0, S - 1, S - 1), radius=S * 0.22, fill=BLUE)
    k = S / 72  # the phone icon's visible 72 units of its 108-unit canvas

    def p(x, y):
        x = 54 + (x - 54) * 0.78
        y = 54 + (y - 54) * 0.78
        return (x - 18) * k, (y - 18) * k

    r = 5 * 0.78 * k
    d.rounded_rectangle((*p(42, 24), *p(66, 84)), radius=r, fill=WHITE)
    d.rounded_rectangle((*p(24, 42), *p(84, 66)), radius=r, fill=WHITE)
    for tri in (((54, 30), (49.5, 35.5), (58.5, 35.5)), ((54, 78), (49.5, 72.5), (58.5, 72.5)),
                ((30, 54), (35.5, 49.5), (35.5, 58.5)), ((78, 54), (72.5, 49.5), (72.5, 58.5))):
        d.polygon([p(*v) for v in tri], fill=BLUE)
    cx, cy = p(54, 54)
    cr = 7.5 * 0.78 * k
    d.ellipse((cx - cr, cy - cr, cx + cr, cy + cr), fill=ORANGE)
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
