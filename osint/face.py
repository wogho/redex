#!/usr/bin/env python3
"""Face embedding helper for the Redex OSINT module.

Usage: face.py <image-path>
Prints a JSON array: [{bbox, det_score, embedding(normalized 512-d)}]
Model: InsightFace buffalo_l (downloaded at image build time).
"""
import json
import sys

import cv2
from insightface.app import FaceAnalysis

_app = None


def app():
    global _app
    if _app is None:
        _app = FaceAnalysis(name="buffalo_l", providers=["CPUExecutionProvider"])
        _app.prepare(ctx_id=-1, det_size=(640, 640))
    return _app


def main() -> int:
    if len(sys.argv) != 2:
        print(json.dumps({"error": "usage: face.py <image>"}))
        return 2
    img = cv2.imread(sys.argv[1])
    if img is None:
        print(json.dumps({"error": "cannot read image"}))
        return 3
    faces = app().get(img)
    out = []
    for f in faces:
        out.append({
            "bbox": [float(x) for x in f.bbox],
            "det_score": float(f.det_score),
            "embedding": [float(x) for x in f.normed_embedding],
        })
    print(json.dumps(out))
    return 0


if __name__ == "__main__":
    sys.exit(main())
