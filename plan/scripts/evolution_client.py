#!/usr/bin/env python3
"""Minimal Evolution API client: retrieve messages (text + media), download media,
and respond with text + media. Reference implementation / normalization oracle for the
xchats backend.

Tested against evoapicloud/evolution-api:v2.3.7.

Env (or flags):
  EVO_BASE   default http://localhost:9700
  EVO_KEY    apikey (required)
  EVO_INST   instance name, default "xpayment"

Examples:
  EVO_KEY=... python evolution_client.py messages 5200000000000@lid --limit 20
  EVO_KEY=... python evolution_client.py download 3ABA7192D3DC90707512 out.jpg
  EVO_KEY=... python evolution_client.py text 77000000000 "hello"
  EVO_KEY=... python evolution_client.py media 77000000000 ./photo.jpg "caption here"

KEY LID FACTS (v2.3.7):
  - Each message key has BOTH `remoteJid` (often <id>@lid) and `remoteJidAlt`
    (the real <phone>@s.whatsapp.net). Use the phone for a stable contact identity.
  - You can send to either the @lid or the phone; the phone is safest. Sending to a
    @lid was rejected with 400 exists:false on the old v2.2.3 — fixed in 2.3.x.
"""
import argparse
import base64
import json
import os
import sys
import urllib.request

BASE = os.getenv("EVO_BASE", "http://localhost:9700").rstrip("/")
KEY = os.getenv("EVO_KEY", "")
INST = os.getenv("EVO_INST", "xpayment")

MEDIA_TYPES = ("imageMessage", "videoMessage", "audioMessage", "documentMessage", "stickerMessage")


def _req(method, path, body=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(
        BASE + path, data=data, method=method,
        headers={"Content-Type": "application/json", "apikey": KEY},
    )
    try:
        with urllib.request.urlopen(req, timeout=60) as r:
            return json.loads(r.read().decode())
    except urllib.error.HTTPError as e:
        return {"_http_error": e.code, "body": e.read().decode()[:500]}


def normalize(rec):
    """Flatten an Evolution message record into the fields xchats actually needs."""
    key = rec.get("key", {}) or {}
    mtype = rec.get("messageType")
    msg = rec.get("message", {}) or {}
    lid = key.get("remoteJid")
    phone = key.get("remoteJidAlt")  # <-- the real phone JID (v2.3.7)
    # stable contact identity: prefer the phone JID
    contact = phone or (lid if lid and lid.endswith("@s.whatsapp.net") else lid)

    text = None
    media = None
    if mtype == "conversation":
        text = msg.get("conversation")
    elif mtype == "extendedTextMessage":
        text = (msg.get("extendedTextMessage") or {}).get("text")
    elif mtype in MEDIA_TYPES:
        m = msg.get(mtype, {}) or {}
        text = m.get("caption")
        media = {
            "kind": mtype.replace("Message", ""),
            "mimetype": m.get("mimetype"),
            "fileLength": m.get("fileLength"),
            "seconds": m.get("seconds"),
            "fileName": m.get("fileName"),
            "needs_download_by_id": key.get("id"),  # -> `download` command
        }
    return {
        "evolution_message_id": key.get("id"),
        "direction": "out" if key.get("fromMe") else "in",
        "addressing_mode": key.get("addressingMode"),
        "lid_jid": lid,
        "phone_jid": phone,
        "contact": contact,
        "push_name": rec.get("pushName"),
        "type": mtype,
        "text": text,
        "media": media,
        "timestamp": rec.get("messageTimestamp"),
        "status": (rec.get("MessageUpdate") or [{}])[-1].get("status") if rec.get("MessageUpdate") else None,
    }


def cmd_messages(args):
    where = {}
    if args.remote_jid:
        where = {"key": {"remoteJid": args.remote_jid}}
    resp = _req("POST", f"/chat/findMessages/{INST}", {"where": where})
    recs = resp.get("messages", {}).get("records", []) if isinstance(resp, dict) else []
    recs = recs[: args.limit] if args.limit else recs
    out = [normalize(r) for r in recs]
    print(json.dumps(out, indent=2, ensure_ascii=False))
    print(f"\n# {len(out)} messages (total in scope: {resp.get('messages', {}).get('total')})", file=sys.stderr)


def cmd_download(args):
    resp = _req("POST", f"/chat/getBase64FromMediaMessage/{INST}",
                {"message": {"key": {"id": args.message_id}}, "convertToMp4": False})
    if "base64" not in resp:
        print("download failed:", json.dumps(resp)[:400]); sys.exit(1)
    raw = base64.b64decode(resp["base64"])
    out = args.out or resp.get("fileName", f"{args.message_id}.bin")
    open(out, "wb").write(raw)
    print(f"saved {out}  ({len(raw)} bytes, {resp.get('mimetype')})")


def cmd_text(args):
    resp = _req("POST", f"/message/sendText/{INST}", {"number": args.number, "text": args.text})
    print(json.dumps({"status": resp.get("status"), "id": (resp.get("key") or {}).get("id"),
                      "err": resp.get("_http_error"), "body": resp.get("body")}, ensure_ascii=False))


def cmd_media(args):
    raw = open(args.file, "rb").read()
    b64 = base64.b64encode(raw).decode()
    ext = os.path.splitext(args.file)[1].lstrip(".").lower()
    kind = "image" if ext in ("jpg", "jpeg", "png", "webp", "gif") else \
           "video" if ext in ("mp4", "mov", "mkv") else \
           "audio" if ext in ("ogg", "mp3", "m4a", "opus") else "document"
    mime = {"jpg": "image/jpeg", "jpeg": "image/jpeg", "png": "image/png",
            "mp4": "video/mp4", "ogg": "audio/ogg", "mp3": "audio/mpeg",
            "pdf": "application/pdf"}.get(ext, "application/octet-stream")
    body = {"number": args.number, "mediatype": kind, "mimetype": mime,
            "media": b64, "fileName": os.path.basename(args.file)}
    if args.caption:
        body["caption"] = args.caption
    resp = _req("POST", f"/message/sendMedia/{INST}", body)
    print(json.dumps({"status": resp.get("status"), "type": resp.get("messageType"),
                      "id": (resp.get("key") or {}).get("id"),
                      "err": resp.get("_http_error"), "body": resp.get("body")}, ensure_ascii=False))


if __name__ == "__main__":
    p = argparse.ArgumentParser(description="Evolution API client (retrieve + respond)")
    sub = p.add_subparsers(dest="cmd", required=True)
    m = sub.add_parser("messages", help="retrieve normalized messages")
    m.add_argument("remote_jid", nargs="?", help="filter by chat remoteJid (lid or phone)")
    m.add_argument("--limit", type=int, default=0)
    m.set_defaults(func=cmd_messages)
    d = sub.add_parser("download", help="download media bytes by message id")
    d.add_argument("message_id"); d.add_argument("out", nargs="?")
    d.set_defaults(func=cmd_download)
    t = sub.add_parser("text", help="send a text reply")
    t.add_argument("number"); t.add_argument("text")
    t.set_defaults(func=cmd_text)
    me = sub.add_parser("media", help="send a media reply from a local file")
    me.add_argument("number"); me.add_argument("file"); me.add_argument("caption", nargs="?")
    me.set_defaults(func=cmd_media)
    args = p.parse_args()
    if not KEY:
        print("set EVO_KEY", file=sys.stderr); sys.exit(2)
    args.func(args)
