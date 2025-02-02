#! /usr/bin/env python3
import sqlite3

db = sqlite3.connect("A:\media.db")
rows = db.execute("SELECT path FROM file")

ext = set()

for x in rows:
    ext.add(x[0].split(".")[-1])

print(ext)