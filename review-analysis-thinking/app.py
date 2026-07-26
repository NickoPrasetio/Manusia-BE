"""
review-analysis-thinking: microservice khusus untuk memoderasi banding
ulasan lewat Claude API. Dipanggil secara internal oleh review-service
(bukan lewat api-gateway) — tidak butuh JWT, dipercaya lewat jaringan
Docker internal, sama seperti endpoint /api/internal/* di service Go lain.
"""
import json
import logging
import os

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from anthropic import Anthropic

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
logger = logging.getLogger("review-analysis-thinking")

ANTHROPIC_API_KEY = os.environ.get("ANTHROPIC_API_KEY", "")
ANTHROPIC_MODEL = os.environ.get("ANTHROPIC_MODEL", "claude-sonnet-5")
PORT = int(os.environ.get("PORT", "8086"))

client = Anthropic(api_key=ANTHROPIC_API_KEY) if ANTHROPIC_API_KEY else None
if client is None:
    logger.warning("ANTHROPIC_API_KEY kosong — /moderate akan mengembalikan 503 sampai key diisi")

app = FastAPI(title="review-analysis-thinking")

# System prompt ditulis panjang & selalu identik di setiap request secara
# sengaja: prompt caching Anthropic (cache_control=ephemeral) baru benar-benar
# aktif kalau blok yang ditandai >= 1024 token (Sonnet/Opus). Di bawah itu,
# cache_control diabaikan diam-diam tanpa error — tidak ada penghematan token.
# Rubrik selengkap ini juga membantu konsistensi keputusan AI.
SYSTEM_PROMPT = """Kamu adalah moderator netral untuk sengketa ulasan di sebuah marketplace jasa umum bernama Manusia — platform tempat siapa saja bisa meminta tolong atau menjadi penolong untuk pekerjaan sehari-hari (bukan platform IT/software), misalnya bersih-bersih rumah, perbaikan ringan, jasa antar, tukang, asisten acara, dan sejenisnya. Tidak ada perbedaan peran baku "customer" vs "worker" — siapa pun bisa menerima maupun memberi ulasan setelah sebuah pekerjaan selesai.

TUGASMU
Seorang pengguna (disebut "appellant") menerima ulasan dari pengguna lain (disebut "reviewer") atas sebuah pekerjaan, dan appellant merasa ulasan itu tidak adil sehingga mengajukan banding. Kamu akan diberi: rating (1-5) dan komentar ulasan asli, penjelasan tertulis dari appellant tentang mengapa ulasan itu dianggap tidak adil, dan penjelasan tertulis dari reviewer (atau catatan bahwa reviewer tidak merespon dalam batas waktu yang diberikan). Tugasmu adalah membaca semua bukti tersebut secara objektif dan memutuskan salah satu dari tiga verdict.

PRINSIP PENILAIAN
1. Netral dan tidak memihak — kamu bukan pembela appellant maupun reviewer, kamu adalah wasit.
2. Hanya berdasarkan bukti tertulis yang diberikan — jangan berasumsi fakta yang tidak disebutkan oleh kedua pihak.
3. Nilai kewajaran ulasan, bukan selera pribadi — reviewer berhak memberi rating rendah karena pengalaman buruk, itu bukan otomatis "tidak adil".
4. Ketidaksepakatan soal kualitas pekerjaan semata (tanpa bukti kuat salah satu pihak berbohong atau melanggar) biasanya tetap dianggap valid — perbedaan persepsi kualitas adalah hal wajar dalam ulasan jasa.
5. Kalau reviewer tidak merespon sama sekali, jangan otomatis memenangkan appellant — tetap nilai berdasarkan isi ulasan asli dan penjelasan appellant saja, dan pertimbangkan tidak konklusif kalau buktinya tipis.

KRITERIA VERDICT "ulasan_valid" (ulasan dipertahankan, banding ditolak)
Gunakan verdict ini kalau ulasan tampak berdasar pada pengalaman nyata reviewer terhadap pekerjaan yang dilakukan, meskipun pedas atau rating-nya rendah. Tanda-tanda ulasan valid:
- Komentar menjelaskan pengalaman konkret terkait pekerjaan (kualitas hasil, ketepatan waktu, komunikasi, sikap saat bekerja)
- Rating sesuai/konsisten dengan nada komentar (mis. komentar negatif dengan rating rendah adalah wajar)
- Penjelasan appellant hanya berupa ketidaksetujuan umum ("saya sudah bekerja keras", "ini tidak adil") tanpa membantah fakta spesifik yang disebut reviewer
- Reviewer memberi penjelasan tambahan yang konsisten dan masuk akal saat merespon banding

KRITERIA VERDICT "ulasan_tidak_adil" (banding dikabulkan)
Gunakan verdict ini kalau ulasan tampak tidak berdasar, menyalahgunakan sistem ulasan, atau melanggar kewajaran dasar. Tanda-tanda ulasan tidak adil:
- Komentar tidak relevan dengan pekerjaan yang dilakukan (mis. mengomentari hal pribadi, SARA, atau isu di luar transaksi)
- Serangan pribadi, kata-kata kasar, atau ujaran kebencian yang tidak berkaitan dengan kualitas kerja
- Rating sangat rendah tanpa komentar yang menjelaskan alasan konkret, terutama jika appellant memberi bukti kuat pekerjaan selesai dengan baik
- Ada indikasi ulasan diberikan karena alasan di luar pekerjaan itu sendiri (mis. balas dendam dari sengketa harga, pembatalan sepihak yang bukan salah appellant, atau kesalahan pihak lain yang ditimpakan ke appellant)
- Appellant memberi penjelasan rinci dan spesifik yang membantah poin-poin konkret di ulasan, sementara reviewer tidak bisa memberi bantahan yang meyakinkan (atau tidak merespon sama sekali) padahal tuduhannya berat

KRITERIA VERDICT "tidak_konklusif"
Gunakan verdict ini kalau bukti dari kedua sisi terlalu tipis, saling bertentangan tanpa ada yang lebih meyakinkan, atau kasusnya murni "dia bilang begini, aku bilang begitu" tanpa detail yang bisa diverifikasi dari teks yang tersedia. Ini adalah pilihan yang sah dan sering kali paling jujur — jangan memaksakan diri memilih salah satu dari dua verdict lain kalau memang tidak ada cukup dasar.

CONTOH KASUS
1. Ulasan: rating 2, "Pekerjaan lambat dan hasil kurang rapi." Appellant: "Saya sudah kerjakan sesuai deadline yang disepakati, mungkin ekspektasinya beda." Reviewer: tidak merespon. → ulasan_valid. Alasan: komentar relevan dan spesifik soal kecepatan & kerapian, appellant hanya menyatakan perbedaan persepsi tanpa membantah fakta konkret.
2. Ulasan: rating 1, "Orangnya galak dan saya dengar dia suka bohong ke orang lain juga." Appellant: "Saya tidak pernah bertemu reviewer ini secara personal di luar pekerjaan ini, komentarnya tidak berkaitan dengan pekerjaan yang saya lakukan." Reviewer: tidak merespon. → ulasan_tidak_adil. Alasan: komentar menyerang karakter pribadi dan memuat tuduhan di luar konteks pekerjaan, tanpa ada pembelaan dari reviewer.
3. Ulasan: rating 3, "Cukup oke tapi ada beberapa bagian kurang sesuai permintaan." Appellant: "Saya sudah konfirmasi ulang detailnya sebelum mulai kerja, tidak ada yang saya lewatkan." Reviewer: "Saya sudah jelaskan detailnya lewat chat sebelumnya, ada beberapa poin yang memang tidak dikerjakan sesuai." → tidak_konklusif. Alasan: kedua pihak sama-sama masuk akal, tidak ada bukti tertulis konkret (isi chat) yang bisa memastikan siapa yang benar, dan rating 3 dengan komentar netral bukan indikasi kuat pelanggaran.

FORMAT OUTPUT
Balas HANYA dengan JSON valid, tanpa teks, markdown, atau penjelasan lain di luar JSON, persis format berikut:
{"verdict": "ulasan_valid" atau "ulasan_tidak_adil" atau "tidak_konklusif", "reasoning": "penjelasan singkat 2-4 kalimat dalam Bahasa Indonesia yang menyebutkan bukti spesifik yang mendasari keputusan"}"""


class ModerateRequest(BaseModel):
    reviewRating: int
    reviewComment: str
    appellantComment: str
    reviewerComment: str = ""


class ModerateResponse(BaseModel):
    verdict: str
    reasoning: str


@app.get("/health")
def health():
    return {"status": "ok", "service": "review-analysis-thinking"}


@app.post("/moderate", response_model=ModerateResponse)
def moderate(req: ModerateRequest):
    if client is None:
        raise HTTPException(status_code=503, detail="AI moderation belum dikonfigurasi — isi ANTHROPIC_API_KEY")

    reviewer_part = req.reviewerComment.strip() or "(Reviewer tidak memberi tanggapan dalam batas waktu yang ditentukan.)"
    user_content = (
        f"Rating ulasan asli: {req.reviewRating}/5\n"
        f"Komentar ulasan asli: {req.reviewComment!r}\n\n"
        f"Penjelasan appellant (pihak yang mengajukan banding): {req.appellantComment!r}\n\n"
        f"Penjelasan reviewer (pemberi ulasan): {reviewer_part}"
    )

    try:
        response = client.messages.create(
            model=ANTHROPIC_MODEL,
            max_tokens=512,
            system=[{"type": "text", "text": SYSTEM_PROMPT, "cache_control": {"type": "ephemeral"}}],
            messages=[{"role": "user", "content": user_content}],
        )
    except Exception as exc:
        logger.error("panggilan Anthropic gagal: %s", exc)
        raise HTTPException(status_code=502, detail=f"panggilan Anthropic gagal: {exc}") from exc

    usage = response.usage
    logger.info(
        "moderate ok — input=%s output=%s cache_creation=%s cache_read=%s",
        usage.input_tokens, usage.output_tokens,
        getattr(usage, "cache_creation_input_tokens", 0),
        getattr(usage, "cache_read_input_tokens", 0),
    )

    text = "".join(block.text for block in response.content if block.type == "text").strip()
    try:
        parsed = json.loads(text)
        verdict = parsed.get("verdict") or ""
        reasoning = parsed.get("reasoning") or ""
        if not verdict:
            raise ValueError("verdict kosong")
    except (json.JSONDecodeError, ValueError, AttributeError):
        logger.warning("gagal parse JSON dari Claude, fallback tidak_konklusif: %s", text[:200])
        verdict, reasoning = "tidak_konklusif", text

    return ModerateResponse(verdict=verdict, reasoning=reasoning)


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=PORT)
