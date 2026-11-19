"""Draws the app icon: an eye, for "who is watching". Run with Python and Pillow
from the repository root; the results are committed in assets/.

    python scripts/make-icons.py
"""
from PIL import Image, ImageDraw

SIZE = 256
GREEN = (63, 154, 110, 255)
DARK = (18, 18, 18, 255)
WHITE = (240, 240, 240, 255)


def eye(draw, colour, pupil, scale, offset=(0, 0)):
    ox, oy = offset
    # The eye is two arcs meeting at the corners, drawn as a lens-shaped polygon.
    left, right, top, bottom = 28 * scale, 228 * scale, 70 * scale, 186 * scale
    middle = (top + bottom) / 2
    points = []
    steps = 40
    for i in range(steps + 1):
        t = i / steps
        x = left + (right - left) * t
        bulge = 4 * t * (1 - t)
        points.append((x + ox, middle - (middle - top) * bulge + oy))
    for i in range(steps, -1, -1):
        t = i / steps
        x = left + (right - left) * t
        bulge = 4 * t * (1 - t)
        points.append((x + ox, middle + (bottom - middle) * bulge + oy))
    draw.polygon(points, fill=colour)
    r = 40 * scale
    cx, cy = 128 * scale + ox, middle + oy
    draw.ellipse((cx - r, cy - r, cx + r, cy + r), fill=pupil)


def coloured():
    image = Image.new('RGBA', (SIZE, SIZE), (0, 0, 0, 0))
    draw = ImageDraw.Draw(image)
    draw.rounded_rectangle((0, 0, SIZE - 1, SIZE - 1), radius=48, fill=GREEN)
    eye(draw, WHITE, DARK, 1)
    return image


def template():
    # macOS menu bars draw a template icon in the bar's own colour, from its alpha.
    image = Image.new('RGBA', (SIZE, SIZE), (0, 0, 0, 0))
    draw = ImageDraw.Draw(image)
    eye(draw, (0, 0, 0, 255), (0, 0, 0, 0), 1)
    return image


icon = coloured()
icon.save('assets/icon.png')
icon.resize((64, 64), Image.LANCZOS).save('assets/tray.png')
icon.save('assets/icon.ico', sizes=[(16, 16), (24, 24), (32, 32), (48, 48), (64, 64), (256, 256)])
template().resize((44, 44), Image.LANCZOS).save('assets/tray-template.png')

# The Windows executable's own icon and description, as resource files that
# `go build` picks up from the main package's directory:
#
#     go run github.com/tc-hib/go-winres@v0.3.3 simply --arch amd64,arm64 \
#       --icon assets/icon.ico --manifest gui --product-name dgg-obs-tools \
#       --file-description "OBS overlays for destiny.gg chat"
