# How-to-use

Step-by-step guide to set up and use the Wagram bridge.

---

## 1. Create a Telegram Bot

1. Open Telegram and search for [@BotFather](https://t.me/BotFather)
2. Send `/newbot`
3. Choose a name (e.g. `Wagram Bridge`)
4. Choose a username (e.g. `my_wagram_bot`)
5. Copy the **API token** — you'll need it next

```
Use this token to access the HTTP API:
7123456789:AAHxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

---

## 2. Install and Run

```bash
git clone https://github.com/0xtbug/Wagram.git
cd Wagram
go mod download
```

Set your bot token and start:

**Linux / macOS:**
```bash
export TELEGRAM_BOT_TOKEN="your-token-here"
go run ./cmd/wagram
```

**Windows (PowerShell):**
```powershell
$env:TELEGRAM_BOT_TOKEN = "your-token-here"
go run .\cmd\wagram\main.go
```

You should see:

```
Starting telegram-wa bridge...
Telegram bot started: @my_wagram_bot
Bridge is running. Press Ctrl+C to exit.
```

---

## 3. Log In to WhatsApp

1. Open Telegram and send `/scan` to your bot
2. The bot will reply with a QR code image
3. Open WhatsApp on your phone → **Settings** → **Linked Devices** → **Link a Device**
4. Scan the QR code
5. The bot will confirm: `✅ WhatsApp login successful!`

> Your session is saved to `wa_session.db`. You won't need to scan again unless you log out.

---

## 4. Bridge a Chat

### Private Chat (1-on-1)

1. Create a Telegram group or use your DM with the bot
2. Send `/bridge`
3. The bot shows an inline keyboard with your recent WhatsApp contacts
4. Tap the contact you want to bridge
5. Done! Messages now flow both ways

### Verify

Send `/status` to check the connection:

```
📊 Bridge Status

WhatsApp Connected: true
WhatsApp Logged In: true
```

Send `/list` to see active bridges:

```
🔗 Active Bridges:

1. WA: 628123456@s.whatsapp.net ↔ TG: -100123456
```

---

## 5. Sending Messages

Once bridged, messages are forwarded automatically:

| Action | Result |
|--------|--------|
| Send text in TG group | Forwarded to WA contact |
| Send photo/video/doc in TG | Media forwarded to WA |
| WA contact sends text | Appears in TG group |
| WA contact sends media | Media appears in TG group |

Messages include sender attribution:

```
WhatsApp: Hello from WhatsApp!
```

```
Telegram: Hello from Telegram!
```

---

## 6. Unbridge a Chat

Send `/unbridge` in the Telegram group to remove the link.

```
✅ Bridge removed for this chat.
```

---

## 7. Managing Multiple Bridges

You can create multiple bridges:

1. Create separate Telegram groups for each WhatsApp contact
2. Run `/bridge` in each group and select a different contact
3. Use `/list` to see all active bridges

> One WhatsApp contact can be bridged to multiple Telegram groups, but each Telegram group can only be linked to one WhatsApp contact.

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| Bot doesn't respond | Check `TELEGRAM_BOT_TOKEN` is set correctly |
| `/scan` shows error | Make sure no other WhatsApp Web session is blocking |
| QR code expired | Send `/scan` again for a new QR code |
| No contacts in `/bridge` | Send a message to someone on WhatsApp first, contacts are discovered dynamically |
| `/status` shows `false` | Run `/scan` to re-login |
| Media not forwarding | Check file size limits (Telegram: 50MB for bot uploads) |

---
