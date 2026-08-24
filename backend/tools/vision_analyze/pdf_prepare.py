"""Prepare bounded, OCR-friendly image segments from a PDF.

The renderer deliberately owns PDF splitting so the vision request never sends
the original PDF to providers whose OpenAI-compatible API only accepts images.
Whitespace is trimmed before splitting.  Only pages that visually resemble a
table are split; ordinary documents stay as one page image.
"""
import os
import sys

import fitz
from PIL import Image


def non_white_bounds(pix):
    """Return the content bounds, preserving a small margin for table lines."""
    samples, width, height, stride = pix.samples, pix.width, pix.height, pix.stride
    rows = []
    for y in range(height):
        row = samples[y * stride:(y + 1) * stride]
        if any(value < 245 for value in row):
            rows.append(y)
    if not rows:
        return 0, height
    return max(0, rows[0] - 12), min(height, rows[-1] + 13)


def is_table(pix):
    """Detect ruled tables from repeated dark horizontal and vertical strokes.

    This deliberately uses image geometry, not attendance-specific words, so
    invoices and other tabular PDFs benefit while prose stays intact.
    """
    samples, width, height, stride = pix.samples, pix.width, pix.height, pix.stride
    horizontal = len(horizontal_line_groups(pix))
    vertical = 0
    for x in range(width):
        dark = 0
        for y in range(0, height, 2):
            if samples[y * stride + x * 3] < 120:
                dark += 1
        if dark >= (height / 2) * 0.20:
            vertical += 1
    return horizontal >= 6 and vertical >= 2


def horizontal_line_groups(pix):
    """Find repeated horizontal table rules and collapse thick lines."""
    samples, width, height, stride = pix.samples, pix.width, pix.height, pix.stride
    candidates = []
    for y in range(height):
        dark = sum(
            1 for x in range(width)
            if samples[y * stride + x * 3] < 160
        )
        if dark >= width * 0.20:
            candidates.append(y)
    groups = []
    for y in candidates:
        if not groups or y > groups[-1][-1] + 2:
            groups.append([y])
        else:
            groups[-1].append(y)
    return [int(sum(group) / len(group)) for group in groups]


def save_table_segments(image, start, end, lines, target, page_number):
    """Compose one row into left/right requests with repeated context columns."""
    header_height = max(80, int((end - start) * 0.12))
    header = image.crop((0, start, image.width, min(end, start + header_height)))
    row_lines = [line for line in lines if start + header_height - 8 <= line <= end]
    boundaries = [start] + row_lines + [end]
    rows = [
        (top, bottom)
        for top, bottom in zip(boundaries, boundaries[1:])
        if bottom - top >= 18
    ]
    if len(rows) < 2:
        return False
    for segment_index, selected in enumerate(rows):
        body = image.crop((0, selected[0], image.width, selected[1]))
        context_width = int(image.width * 0.42)
        split_x = int(image.width * 0.56)
        for side, (header_part, body_part) in {
            "left": (
                header.crop((0, 0, split_x, header.height)),
                body.crop((0, 0, split_x, body.height)),
            ),
            "right": (
                Image.new("RGB", (context_width + image.width - split_x, header.height), "white"),
                Image.new("RGB", (context_width + image.width - split_x, body.height), "white"),
            ),
        }.items():
            if side == "right":
                header_part.paste(header.crop((0, 0, context_width, header.height)), (0, 0))
                header_part.paste(header.crop((split_x, 0, image.width, header.height)), (context_width, 0))
                body_part.paste(body.crop((0, 0, context_width, body.height)), (0, 0))
                body_part.paste(body.crop((split_x, 0, image.width, body.height)), (context_width, 0))
            canvas = Image.new(
                "RGB",
                (header_part.width, header_part.height + body_part.height),
                "white",
            )
            canvas.paste(header_part, (0, 0))
            canvas.paste(body_part, (0, header_part.height))
            canvas.save(os.path.join(
                target,
                f"page-{page_number}-row-{segment_index + 1}-{side}.png",
            ))
    return True


source, target, limit = sys.argv[1], sys.argv[2], int(sys.argv[3])
document = fitz.open(source)
if len(document) > limit:
    raise RuntimeError("PDF exceeds page limit")
for page_number, page in enumerate(document, 1):
    full = page.get_pixmap(dpi=144, alpha=False)
    start, end = non_white_bounds(full)
    image = Image.frombytes("RGB", (full.width, full.height), full.samples)
    # Keep one complete image per original page. Splitting payroll rows or
    # date columns loses column alignment and causes names/days to drift.
    image.crop((0, start, image.width, end)).save(
        os.path.join(target, f"page-{page_number}-part-1.png")
    )
