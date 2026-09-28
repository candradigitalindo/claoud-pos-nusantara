"""
Layanan analitik Analisa Bisnis (FastAPI).

Dipanggil oleh API Go setiap kali halaman Analisa Bisnis dibuka. Tidak
menyentuh basis data: Go mengirim deret yang sudah bersih (setara kalender),
layanan ini mengembalikan angka — tren, perkiraan, minggu aneh, pergeseran
level, pola hari — dan Go yang merakit kalimatnya. Kalau layanan ini mati,
halaman tetap tampil tanpa bagian model.
"""
from fastapi import FastAPI
from pydantic import BaseModel, Field
from typing import Any

from stats import analyze

app = FastAPI(title="cloud-pos analytics", version="1.0")


class Series(BaseModel):
    weeks: list[dict[str, Any]] = Field(default_factory=list)
    daily: list[dict[str, Any]] = Field(default_factory=list)
    future: list[dict[str, Any]] = Field(default_factory=list)
    # Jendela pola hari (hari), mengikuti panjang blok pembanding di Go.
    # Pydantic membuang kolom yang tidak dideklarasikan, jadi harus ada di sini.
    dow_days: int = 28


class AnalyzeRequest(BaseModel):
    group: Series | None = None
    outlets: dict[str, Series] = Field(default_factory=dict)


@app.get("/health")
def health():
    return {"ok": True, "engine": "theil-sen/mad"}


@app.post("/analyze")
def analyze_endpoint(req: AnalyzeRequest):
    out = {"engine": "theil-sen/mad", "group": None, "outlets": {}}
    if req.group is not None:
        out["group"] = analyze(req.group.model_dump())
    for code, s in req.outlets.items():
        out["outlets"][code] = analyze(s.model_dump())
    return out
