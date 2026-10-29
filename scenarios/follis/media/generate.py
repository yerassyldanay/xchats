#!/usr/bin/env python3
"""Generate the Follis test catalog's product photos and demo videos.

Reads ../data/kb.json and writes, per product, `media.images` JPG product cards
(a drawn tool illustration with the name, brand and category) and, when
`media.video` is true, a short H.264 MP4 demo clip built from those cards.
Everything is drawn locally (Pillow + ffmpeg): no stock photos, no network.
The output is committed under media/out/ so a run needs neither tool.

Usage: python3 scenarios/follis/media/generate.py
"""
import json
import math
import os
import subprocess
import sys
import tempfile

from PIL import Image, ImageDraw, ImageFilter, ImageFont

HERE = os.path.dirname(os.path.abspath(__file__))
OUT = os.path.join(HERE, "out")
KB = os.path.join(HERE, "..", "data", "kb.json")
FONT_BOLD = "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf"
FONT = "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"
W, H = 1200, 900

CATEGORY_COLORS = {
    "Наборы инструментов": ("#1d3557", "#e63946"),
    "Ручной инструмент": ("#264653", "#e9c46a"),
    "Пневмоинструмент": ("#2b2d42", "#4cc9f0"),
    "Компрессоры": ("#3a0ca3", "#f72585"),
    "Домкраты и подъёмники": ("#7f1d1d", "#fbbf24"),
    "Шиномонтажное оборудование": ("#14213d", "#fca311"),
    "Диагностика": ("#073b4c", "#06d6a0"),
    "Гаражное оборудование": ("#3d405b", "#f2cc8f"),
}

METAL = "#c9d1d9"
DARK = "#1f2933"


def font(path, size):
    return ImageFont.truetype(path, size)


def gradient(top, bottom):
    img = Image.new("RGB", (W, H), top)
    t = Image.new("RGB", (W, H), bottom)
    mask = Image.linear_gradient("L").resize((W, H))
    return Image.composite(t, img, mask)


def rr(d, box, r, fill, outline=None, width=0):
    d.rounded_rectangle(box, radius=r, fill=fill, outline=outline, width=width)


# --- tool illustrations: each draws into a box centred on (cx, cy) ----------

def draw_wrench(d, cx, cy, s, accent):
    ang = math.radians(-30)
    def rot(x, y):
        return (cx + x * math.cos(ang) - y * math.sin(ang), cy + x * math.sin(ang) + y * math.cos(ang))
    handle = [rot(-260 * s, -28 * s), rot(200 * s, -28 * s), rot(200 * s, 28 * s), rot(-260 * s, 28 * s)]
    d.polygon(handle, fill=METAL, outline=DARK)
    hx, hy = rot(250 * s, 0)
    d.ellipse([hx - 80 * s, hy - 80 * s, hx + 80 * s, hy + 80 * s], fill=METAL, outline=DARK, width=3)
    d.ellipse([hx - 36 * s, hy - 36 * s, hx + 36 * s, hy + 36 * s], fill=accent)
    tx, ty = rot(-280 * s, 0)
    d.ellipse([tx - 60 * s, ty - 60 * s, tx + 60 * s, ty + 60 * s], fill=METAL, outline=DARK, width=3)
    d.regular_polygon((tx, ty, 30 * s), 6, fill=DARK)
    gx, gy = rot(-40 * s, 0)
    d.rectangle([gx - 70 * s, gy - 14 * s, gx + 70 * s, gy + 14 * s], fill=accent)


def draw_toolset(d, cx, cy, s, accent):
    rr(d, [cx - 330 * s, cy - 200 * s, cx + 330 * s, cy + 200 * s], 30 * s, "#111827", outline=accent, width=6)
    rr(d, [cx - 300 * s, cy - 170 * s, cx + 300 * s, cy + 170 * s], 18 * s, "#374151")
    for row in range(3):
        for col in range(9):
            r = (16 + row * 6) * s
            x = cx - 250 * s + col * 62 * s
            y = cy - 110 * s + row * 80 * s
            d.ellipse([x - r, y - r, x + r, y + r], fill=METAL, outline=DARK, width=2)
            d.ellipse([x - r / 2, y - r / 2, x + r / 2, y + r / 2], fill=DARK)
    d.rectangle([cx - 250 * s, cy + 120 * s, cx + 120 * s, cy + 140 * s], fill=METAL)
    d.ellipse([cx + 110 * s, cy + 100 * s, cx + 170 * s, cy + 160 * s], fill=METAL, outline=DARK, width=2)


def draw_socket(d, cx, cy, s, accent):
    for i, r in enumerate([70, 85, 100, 115]):
        x = cx - 270 * s + i * 175 * s
        y = cy + (20 - i * 10) * s
        rr(d, [x - r * s, y - 120 * s, x + r * s, y + 120 * s], 20 * s, "#4b5563", outline=DARK, width=3)
        d.ellipse([x - r * s, y - 150 * s, x + r * s, y - 90 * s], fill="#6b7280", outline=DARK, width=3)
        d.regular_polygon((x, y - 120 * s, r * 0.55 * s), 6, fill=DARK)
        d.rectangle([x - r * s, y + 40 * s, x + r * s, y + 60 * s], fill=accent)


def draw_screwdriver(d, cx, cy, s, accent):
    for i in range(6):
        x = cx - 250 * s + i * 100 * s
        rr(d, [x - 32 * s, cy - 220 * s, x + 32 * s, cy - 40 * s], 28 * s, accent if i % 2 else "#111827")
        d.rectangle([x - 8 * s, cy - 40 * s, x + 8 * s, cy + 200 * s], fill=METAL)
        d.polygon([(x - 8 * s, cy + 200 * s), (x + 8 * s, cy + 200 * s), (x, cy + 225 * s)], fill=METAL)


def draw_impact(d, cx, cy, s, accent):
    rr(d, [cx - 260 * s, cy - 150 * s, cx + 150 * s, cy + 10 * s], 60 * s, "#111827", outline=accent, width=6)
    d.polygon([(cx - 60 * s, cy), (cx + 60 * s, cy), (cx + 30 * s, cy + 260 * s), (cx - 90 * s, cy + 260 * s)], fill="#1f2937", outline=accent)
    d.rectangle([cx + 150 * s, cy - 100 * s, cx + 250 * s, cy - 40 * s], fill=METAL, outline=DARK, width=3)
    d.rectangle([cx + 250 * s, cy - 82 * s, cx + 300 * s, cy - 58 * s], fill=METAL)
    d.rectangle([cx - 70 * s, cy + 260 * s, cx - 30 * s, cy + 300 * s], fill=METAL)
    d.rectangle([cx - 20 * s, cy + 30 * s, cx + 20 * s, cy + 80 * s], fill=accent)


def draw_hose(d, cx, cy, s, accent):
    d.ellipse([cx - 230 * s, cy - 230 * s, cx + 230 * s, cy + 230 * s], fill="#111827", outline=accent, width=10)
    for r in range(60, 210, 22):
        d.ellipse([cx - r * s, cy - r * s, cx + r * s, cy + r * s], outline="#facc15", width=int(9 * s))
    d.ellipse([cx - 40 * s, cy - 40 * s, cx + 40 * s, cy + 40 * s], fill=METAL)
    d.line([(cx + 200 * s, cy + 100 * s), (cx + 320 * s, cy + 220 * s), (cx + 380 * s, cy + 200 * s)], fill="#facc15", width=int(14 * s))
    d.rectangle([cx + 370 * s, cy + 185 * s, cx + 410 * s, cy + 215 * s], fill=METAL)


def draw_compressor(d, cx, cy, s, accent):
    rr(d, [cx - 330 * s, cy - 40 * s, cx + 330 * s, cy + 170 * s], 100 * s, accent, outline=DARK, width=4)
    rr(d, [cx - 160 * s, cy - 210 * s, cx + 120 * s, cy - 40 * s], 20 * s, "#111827")
    for i in range(5):
        d.rectangle([cx - 140 * s + i * 50 * s, cy - 200 * s, cx - 110 * s + i * 50 * s, cy - 60 * s], fill="#374151")
    d.ellipse([cx + 150 * s, cy - 150 * s, cx + 250 * s, cy - 50 * s], fill="white", outline=DARK, width=4)
    d.line([(cx + 200 * s, cy - 100 * s), (cx + 230 * s, cy - 130 * s)], fill="#dc2626", width=5)
    for x in (-250, 250):
        d.ellipse([cx + x * s - 50 * s, cy + 140 * s, cx + x * s + 50 * s, cy + 240 * s], fill="#111827")


def draw_jack(d, cx, cy, s, accent):
    d.polygon([(cx - 300 * s, cy + 120 * s), (cx + 200 * s, cy + 120 * s), (cx + 160 * s, cy + 20 * s), (cx - 260 * s, cy + 40 * s)], fill=accent, outline=DARK)
    d.line([(cx + 140 * s, cy + 60 * s), (cx - 60 * s, cy - 80 * s)], fill="#111827", width=int(34 * s))
    d.ellipse([cx - 120 * s, cy - 120 * s, cx - 10 * s, cy - 70 * s], fill=METAL, outline=DARK, width=3)
    d.line([(cx - 280 * s, cy + 80 * s), (cx - 460 * s, cy - 200 * s)], fill=METAL, width=int(22 * s))
    for x in (-250, 150):
        d.ellipse([cx + x * s - 40 * s, cy + 100 * s, cx + x * s + 40 * s, cy + 180 * s], fill="#111827")


def draw_bottlejack(d, cx, cy, s, accent):
    rr(d, [cx - 150 * s, cy + 160 * s, cx + 150 * s, cy + 210 * s], 10 * s, "#111827")
    rr(d, [cx - 110 * s, cy - 120 * s, cx + 110 * s, cy + 170 * s], 30 * s, accent, outline=DARK, width=4)
    d.rectangle([cx - 45 * s, cy - 230 * s, cx + 45 * s, cy - 120 * s], fill=METAL, outline=DARK, width=3)
    rr(d, [cx - 70 * s, cy - 260 * s, cx + 70 * s, cy - 230 * s], 8 * s, "#374151")
    d.line([(cx + 110 * s, cy + 80 * s), (cx + 330 * s, cy - 40 * s)], fill=METAL, width=int(18 * s))


def draw_stands(d, cx, cy, s, accent):
    for off in (-180, 180):
        x = cx + off * s
        d.polygon([(x - 120 * s, cy + 200 * s), (x + 120 * s, cy + 200 * s), (x + 30 * s, cy - 60 * s), (x - 30 * s, cy - 60 * s)], fill=accent, outline=DARK)
        d.rectangle([x - 18 * s, cy - 200 * s, x + 18 * s, cy - 60 * s], fill=METAL, outline=DARK)
        d.polygon([(x - 60 * s, cy - 230 * s), (x + 60 * s, cy - 230 * s), (x + 30 * s, cy - 200 * s), (x - 30 * s, cy - 200 * s)], fill="#111827")


def draw_lift(d, cx, cy, s, accent):
    for x in (-330, 330):
        rr(d, [cx + x * s - 35 * s, cy - 260 * s, cx + x * s + 35 * s, cy + 230 * s], 8 * s, accent, outline=DARK, width=3)
    d.rectangle([cx - 330 * s, cy - 270 * s, cx + 330 * s, cy - 240 * s], fill="#111827")
    d.rectangle([cx - 300 * s, cy + 30 * s, cx + 300 * s, cy + 50 * s], fill=METAL)
    body = [(cx - 260 * s, cy + 30 * s), (cx - 220 * s, cy - 40 * s), (cx - 110 * s, cy - 50 * s), (cx - 60 * s, cy - 120 * s),
            (cx + 120 * s, cy - 120 * s), (cx + 190 * s, cy - 50 * s), (cx + 260 * s, cy - 40 * s), (cx + 270 * s, cy + 30 * s)]
    d.polygon(body, fill="#e5e7eb", outline=DARK)
    for x in (-160, 170):
        d.ellipse([cx + x * s - 50 * s, cy - 10 * s, cx + x * s + 50 * s, cy + 90 * s], fill="#111827")


def draw_tire(d, cx, cy, s, accent):
    rr(d, [cx - 220 * s, cy + 40 * s, cx + 220 * s, cy + 240 * s], 20 * s, accent, outline=DARK, width=4)
    d.rectangle([cx + 170 * s, cy - 250 * s, cx + 220 * s, cy + 40 * s], fill="#374151")
    d.rectangle([cx - 40 * s, cy - 250 * s, cx + 220 * s, cy - 215 * s], fill="#374151")
    d.ellipse([cx - 200 * s, cy - 150 * s, cx + 140 * s, cy + 60 * s], fill="#111827")
    d.ellipse([cx - 120 * s, cy - 100 * s, cx + 60 * s, cy + 10 * s], fill=METAL, outline=DARK, width=3)


def draw_balancer(d, cx, cy, s, accent):
    rr(d, [cx - 120 * s, cy - 60 * s, cx + 250 * s, cy + 240 * s], 20 * s, accent, outline=DARK, width=4)
    rr(d, [cx - 80 * s, cy - 220 * s, cx + 210 * s, cy - 80 * s], 14 * s, "#111827")
    for i in range(4):
        d.rectangle([cx - 50 * s + i * 65 * s, cy - 180 * s, cx - 10 * s + i * 65 * s, cy - 130 * s], fill="#22c55e")
    d.rectangle([cx - 260 * s, cy + 60 * s, cx - 120 * s, cy + 80 * s], fill=METAL)
    d.ellipse([cx - 420 * s, cy - 60 * s, cx - 220 * s, cy + 200 * s], fill="#111827")
    d.ellipse([cx - 370 * s, cy, cx - 270 * s, cy + 140 * s], fill=METAL, outline=DARK, width=3)


def draw_scanner(d, cx, cy, s, accent):
    rr(d, [cx - 170 * s, cy - 240 * s, cx + 170 * s, cy + 220 * s], 40 * s, "#111827", outline=accent, width=8)
    rr(d, [cx - 130 * s, cy - 200 * s, cx + 130 * s, cy + 20 * s], 12 * s, "#0ea5e9")
    for i in range(3):
        d.rectangle([cx - 110 * s, cy - 170 * s + i * 55 * s, cx + (40 - i * 30) * s, cy - 145 * s + i * 55 * s], fill="white")
    for i in range(3):
        d.ellipse([cx - 110 * s + i * 80 * s, cy + 70 * s, cx - 50 * s + i * 80 * s, cy + 130 * s], fill="#374151")
    d.line([(cx + 170 * s, cy + 120 * s), (cx + 330 * s, cy + 200 * s), (cx + 360 * s, cy + 120 * s)], fill="#111827", width=int(16 * s))
    rr(d, [cx + 330 * s, cy + 60 * s, cx + 420 * s, cy + 130 * s], 10 * s, accent)


def draw_tester(d, cx, cy, s, accent):
    rr(d, [cx - 140 * s, cy - 200 * s, cx + 140 * s, cy + 160 * s], 30 * s, accent, outline=DARK, width=4)
    rr(d, [cx - 100 * s, cy - 160 * s, cx + 100 * s, cy - 30 * s], 10 * s, "#111827")
    d.text((cx - 70 * s, cy - 140 * s), "12.6V", fill="#22c55e", font=font(FONT_BOLD, int(60 * s)))
    for side, color in ((-1, "#dc2626"), (1, "#111827")):
        d.line([(cx + side * 60 * s, cy + 160 * s), (cx + side * 220 * s, cy + 260 * s)], fill=color, width=int(14 * s))
        d.polygon([(cx + side * 200 * s, cy + 240 * s), (cx + side * 280 * s, cy + 250 * s), (cx + side * 240 * s, cy + 300 * s)], fill=color)


def draw_cabinet(d, cx, cy, s, accent):
    rr(d, [cx - 220 * s, cy - 250 * s, cx + 220 * s, cy + 200 * s], 16 * s, accent, outline=DARK, width=4)
    for i in range(7):
        y = cy - 230 * s + i * 60 * s
        d.rectangle([cx - 195 * s, y, cx + 195 * s, y + 48 * s], fill="#b91c1c" if accent != "#b91c1c" else "#7f1d1d", outline=DARK)
        d.rectangle([cx - 60 * s, y + 18 * s, cx + 60 * s, y + 28 * s], fill=METAL)
    for x in (-180, 180):
        d.ellipse([cx + x * s - 30 * s, cy + 200 * s, cx + x * s + 30 * s, cy + 260 * s], fill="#111827")


def draw_workbench(d, cx, cy, s, accent):
    d.rectangle([cx - 360 * s, cy - 80 * s, cx + 360 * s, cy - 40 * s], fill=METAL, outline=DARK, width=3)
    d.rectangle([cx - 340 * s, cy - 40 * s, cx - 310 * s, cy + 230 * s], fill="#374151")
    rr(d, [cx + 100 * s, cy - 40 * s, cx + 340 * s, cy + 230 * s], 8 * s, accent, outline=DARK, width=3)
    for i in range(4):
        d.rectangle([cx + 115 * s, cy - 25 * s + i * 62 * s, cx + 325 * s, cy + 25 * s + i * 62 * s], outline=DARK, width=3)
    d.rectangle([cx - 250 * s, cy - 200 * s, cx - 120 * s, cy - 80 * s], fill="#111827")


def draw_oil(d, cx, cy, s, accent):
    rr(d, [cx - 150 * s, cy - 80 * s, cx + 150 * s, cy + 200 * s], 40 * s, accent, outline=DARK, width=4)
    d.rectangle([cx - 18 * s, cy - 300 * s, cx + 18 * s, cy - 80 * s], fill=METAL, outline=DARK)
    d.polygon([(cx - 160 * s, cy - 360 * s), (cx + 160 * s, cy - 360 * s), (cx + 30 * s, cy - 290 * s), (cx - 30 * s, cy - 290 * s)], fill="#facc15", outline=DARK)
    for x in (-110, 110):
        d.ellipse([cx + x * s - 35 * s, cy + 190 * s, cx + x * s + 35 * s, cy + 260 * s], fill="#111827")


ICONS = {
    "wrench": draw_wrench, "toolset": draw_toolset, "socket": draw_socket, "screwdriver": draw_screwdriver,
    "impact": draw_impact, "hose": draw_hose, "compressor": draw_compressor, "jack": draw_jack,
    "bottlejack": draw_bottlejack, "stands": draw_stands, "lift": draw_lift, "tire": draw_tire,
    "balancer": draw_balancer, "scanner": draw_scanner, "tester": draw_tester, "cabinet": draw_cabinet,
    "workbench": draw_workbench, "oil": draw_oil,
}


def wrap(d, text, fnt, width):
    words, lines, cur = text.split(), [], ""
    for w in words:
        trial = (cur + " " + w).strip()
        if d.textlength(trial, font=fnt) <= width:
            cur = trial
        else:
            lines.append(cur)
            cur = w
    if cur:
        lines.append(cur)
    return lines


def card(product, variant):
    bg, accent = CATEGORY_COLORS.get(product["category"], ("#1f2937", "#f59e0b"))
    img = gradient(bg, "#0b0f14")
    # soft spotlight behind the tool
    glow = Image.new("L", (W, H), 0)
    ImageDraw.Draw(glow).ellipse([250, 90, 950, 650], fill=150)
    glow = glow.filter(ImageFilter.GaussianBlur(120))
    img = Image.composite(Image.new("RGB", (W, H), "#e2e8f0"), img, glow.point(lambda v: v // 3))
    d = ImageDraw.Draw(img)

    draw = ICONS[product["icon"]]
    if variant == 0:
        draw(d, W // 2, 380, 1.0, accent)
    else:  # second angle: closer and off-centre, like a detail shot
        draw(d, W // 2 + 120, 400, 1.3, accent)

    d.rectangle([0, H - 210, W, H], fill="#0b0f14")
    d.rectangle([0, H - 210, 14, H], fill=accent)
    name_font = font(FONT_BOLD, 44)
    y = H - 190
    for line in wrap(d, product["name"], name_font, W - 120)[:2]:
        d.text((50, y), line, fill="white", font=name_font)
        y += 54
    d.text((50, y + 6), f'{product.get("brand", "")} · {product["category"]}', fill="#cbd5e1", font=font(FONT, 28))
    d.text((W - 300, 34), "FOLLIS", fill="white", font=font(FONT_BOLD, 54))
    rr(d, [40, 34, 330, 84], 25, "#fbbf24")
    d.text((62, 44), "ТЕСТОВОЕ ФОТО", fill="#111827", font=font(FONT_BOLD, 26))
    if variant == 1:
        d.text((50, 110), "вид крупным планом", fill="#e2e8f0", font=font(FONT, 28))
    return img


def video(product, frames, dest):
    with tempfile.TemporaryDirectory() as tmp:
        inputs = []
        for i, f in enumerate(frames):
            p = os.path.join(tmp, f"f{i}.jpg")
            f.save(p, quality=92)
            inputs += ["-loop", "1", "-t", "4", "-i", p]
        n = len(frames)
        parts = [f"[{i}:v]scale=1600:1200,zoompan=z='min(zoom+0.0012,1.15)':d=100:s=1280x960:fps=25,setsar=1[v{i}]" for i in range(n)]
        if n == 1:
            graph = parts[0] + ";[v0]format=yuv420p[out]"
        else:
            graph = ";".join(parts) + ";[v0][v1]xfade=transition=fade:duration=0.8:offset=3.2,format=yuv420p[out]"
        cmd = ["ffmpeg", "-y", "-loglevel", "error", *inputs, "-filter_complex", graph, "-map", "[out]",
               "-c:v", "libx264", "-preset", "medium", "-crf", "30", "-movflags", "+faststart", "-t", "7", dest]
        subprocess.run(cmd, check=True)


def main():
    with open(KB, encoding="utf-8") as f:
        kb = json.load(f)
    os.makedirs(OUT, exist_ok=True)
    manifest = {}
    for p in kb["products"]:
        media = p.get("media", {})
        files = {"images": [], "videos": []}
        frames = []
        for i in range(media.get("images", 0)):
            img = card(p, i % 2)
            name = f'{p["ref"]}-{i + 1}.jpg'
            img.save(os.path.join(OUT, name), quality=82, optimize=True)
            files["images"].append(name)
            frames.append(img)
        if media.get("video"):
            name = f'{p["ref"]}-demo.mp4'
            video(p, frames or [card(p, 0)], os.path.join(OUT, name))
            files["videos"].append(name)
        manifest[p["ref"]] = files
        print(p["ref"], files, file=sys.stderr)
    with open(os.path.join(OUT, "manifest.json"), "w", encoding="utf-8") as f:
        json.dump(manifest, f, ensure_ascii=False, indent=2)


if __name__ == "__main__":
    main()
