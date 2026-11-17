#!/usr/bin/env python3
"""Draws the app icon (needs Pillow): build/win/aiot-simulator.ico and build/aiot-simulator.png"""
import sys
from PIL import Image, ImageDraw
out = sys.argv[1] if len(sys.argv) > 1 else "build"
S = 512; im = Image.new("RGBA", (S, S), (0, 0, 0, 0)); d = ImageDraw.Draw(im)
d.rounded_rectangle([0, 0, S-1, S-1], radius=S*0.22, fill=(15, 42, 61, 255))
c = S/2; teal = (45, 212, 191, 255); blue = (56, 189, 248, 255); w = int(S*.047)
for l in [(c,S*.36,c,S*.16),(c,S*.64,c,S*.84),(S*.36,c,S*.16,c),(S*.64,c,S*.84,c)]: d.line(l, fill=blue, width=w)
for l in [(S*.41,S*.41,S*.27,S*.27),(S*.73,S*.73,S*.59,S*.59),(S*.59,S*.41,S*.73,S*.27),(S*.27,S*.73,S*.41,S*.59)]: d.line(l, fill=teal, width=w)
r = S*.14; d.ellipse([c-r,c-r,c+r,c+r], fill=teal)
for x, y in [(c,S*.14),(c,S*.86),(S*.14,c),(S*.86,c)]:
    q = S*.055; d.ellipse([x-q,y-q,x+q,y+q], fill=blue)
im.save(f"{out}/win/aiot-simulator.ico", sizes=[(256,256),(128,128),(64,64),(48,48),(32,32),(16,16)])
im.resize((256,256)).save(f"{out}/aiot-simulator.png")
