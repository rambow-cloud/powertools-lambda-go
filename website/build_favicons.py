"""Package the approved favicon artwork; run with uv run --with pillow."""

from pathlib import Path

from PIL import Image


ROOT = Path(__file__).resolve().parents[1]
with Image.open(ROOT / "website/branding/favicon-source.png") as source:
    icon = source.convert("RGBA")
    if icon.width != icon.height:
        raise SystemExit("Favicon artwork must be square.")
    icon.resize((96, 96), Image.Resampling.LANCZOS).save(ROOT / "docs/favicon.png", optimize=True)
    icon.resize((256, 256), Image.Resampling.LANCZOS).save(
        ROOT / "docs/favicon.ico", sizes=[(16, 16), (32, 32), (48, 48), (256, 256)]
    )
print("Packaged docs/favicon.png (96px) and docs/favicon.ico (16/32/48/256px).")
