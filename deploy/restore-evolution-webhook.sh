#!/usr/bin/env bash
# RESTORE POINT — the per-instance Evolution webhook for instance "xpayment"
# BEFORE xchats repointed it to its own ingress (captured 2026-06-15).
#
# Context: the live Chatwoot/kaspi flow is fed by Evolution's GLOBAL webhook
# (WEBHOOK_GLOBAL_URL=http://evolution-webhook:9701/evolution), which is
# independent of this per-instance webhook and was the only active feed.
# xchats repurposes the (dormant) per-instance slot. Run this to put it back.
set -euo pipefail
EVO_BASE="${EVOLUTION_BASE_URL:-http://localhost:9700}"
KEY="${EVOLUTION_API_KEY:-019a63bf-aedc-7603-a7f5-c891bb2d7145}"
INSTANCE="${EVOLUTION_INSTANCE:-xpayment}"

curl -fsS -X POST "$EVO_BASE/webhook/set/$INSTANCE" \
  -H "apikey: $KEY" -H 'Content-Type: application/json' \
  -d '{"webhook":{
    "enabled": true,
    "url": "http://host.docker.internal:9701",
    "webhookByEvents": true,
    "webhookBase64": true,
    "headers": null,
    "events": ["APPLICATION_STARTUP","CALL","CHATS_DELETE","CHATS_SET","CHATS_UPDATE","CHATS_UPSERT","CONNECTION_UPDATE","CONTACTS_SET","CONTACTS_UPDATE","CONTACTS_UPSERT","GROUP_PARTICIPANTS_UPDATE","GROUP_UPDATE","GROUPS_UPSERT","LABELS_ASSOCIATION","LABELS_EDIT","LOGOUT_INSTANCE","MESSAGES_DELETE","MESSAGES_SET","MESSAGES_UPDATE","MESSAGES_UPSERT","PRESENCE_UPDATE","QRCODE_UPDATED","REMOVE_INSTANCE","SEND_MESSAGE","TYPEBOT_CHANGE_STATUS","TYPEBOT_START"]
  }}'
echo
echo "Restored per-instance webhook for $INSTANCE -> http://host.docker.internal:9701"
