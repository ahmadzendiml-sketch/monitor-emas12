package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func sendLogToAdmin(bot *tgbotapi.BotAPI, user *tgbotapi.User, command, args, status string) {
	adminIDstr := os.Getenv("ADMIN_CHAT_ID")
	if adminIDstr == "" {
		return
	}
	adminID, err := strconv.ParseInt(adminIDstr, 10, 64)
	if err != nil {
		return
	}
	username := user.UserName
	if username == "" {
		username = "tidak ada"
	}
	fullName := strings.TrimSpace(user.FirstName + " " + user.LastName)
	if fullName == "" {
		fullName = "N/A"
	}
	logMsg := fmt.Sprintf(
		"📋 <b>Command Log</b> %s\n━━━━━━━━━━━━━━━━━━━\n👤 <b>Nama:</b> %s\n🆔 <b>User ID:</b> <code>%d</code>\n📛 <b>Username:</b> @%s\n📝 <b>Command:</b> /%s\n",
		status, fullName, user.ID, username, command,
	)
	if args != "" {
		logMsg += fmt.Sprintf("📄 <b>Args:</b> %s\n", args)
	}
	logMsg += fmt.Sprintf("⏰ <b>Waktu:</b> %s WIB", time.Now().In(time.FixedZone("WIB", 7*3600)).Format("2006-01-02 15:04:05"))
	msg := tgbotapi.NewMessage(adminID, logMsg)
	msg.ParseMode = "HTML"
	bot.Send(msg)
}

func StartTelegramBot() {
	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		return
	}
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return
	}
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)
	adminID, _ := strconv.ParseInt(os.Getenv("ADMIN_CHAT_ID"), 10, 64)
	for update := range updates {
		if update.Message == nil {
			continue
		}
		userID := update.Message.From.ID
		text := update.Message.Text
		user := update.Message.From

		// Ban check
		if banned[userID] {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "⛔ Anda telah dibanned. MAMPUSS dahh akkwkwkwkw😂😂😂.")
			bot.Send(msg)
			continue
		}

		// /start
		if strings.HasPrefix(text, "/start") {
			sendLogToAdmin(bot, user, "start", "", "✅")
			var helpText string
			if int64(userID) == adminID {
				helpText = "🤖 <b>Bot aktif!</b> (Admin Mode)\n\n" +
					"<b>📌 Perintah User:</b>\n" +
					"━━━━━━━━━━━━━━━━━━━\n" +
					"⏰ /in &lt;jam&gt; - Input jam transfer (cth: /in 09.30)\n" +
					"ℹ️ /myid - Lihat ID Telegram Anda\n" +
					"\n<b>👑 Perintah Admin:</b>\n" +
					"━━━━━━━━━━━━━━━━━━━\n" +
					"📝 /atur &lt;teks&gt; - Ubah info Treasury\n" +
					"🔄 /resetjam - Reset data transfer\n" +
					"🚫 /banid &lt;id&gt; - Ban user by ID\n" +
					"✅ /unbanid &lt;id&gt; - Unban user by ID\n" +
					"📋 /listban - Lihat daftar user banned\n"
			} else {
				helpText = "🤖 <b>Bot aktif!</b>\n\n" +
					"<b>Cara gunakan:</b>\n" +
					"⏰ /in &lt;jam&gt; - Input jam transfer\n" +
					"   Contoh: /in 09.30\n" +
					"ℹ️ /myid - Lihat ID Telegram Anda\n"
			}
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, helpText)
			msg.ParseMode = "HTML"
			bot.Send(msg)
			continue
		}

		// /myid
		if strings.HasPrefix(text, "/myid") {
			sendLogToAdmin(bot, user, "myid", "", "✅")
			userInfo := fmt.Sprintf(
				"ℹ️ <b>Informasi Akun </b>\n🆔 <b>User ID:</b> <code>%d</code>\n👤 <b>Nama:</b> %s %s\n📛 <b>Username:</b> @%s\n",
				userID, user.FirstName, user.LastName, user.UserName,
			)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, userInfo)
			msg.ParseMode = "HTML"
			bot.Send(msg)
			continue
		}

		// /in <jam>
		if strings.HasPrefix(text, "/in") {
			jam := strings.TrimSpace(strings.TrimPrefix(text, "/in"))
			jam = strings.ReplaceAll(jam, ".", ":")
			jam = strings.ReplaceAll(jam, ",", ":")
			sendLogToAdmin(bot, user, "in", jam, "✅")
			if jam == "" {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Gunakan: /in <jam>\nContoh: /in 09.30")
				bot.Send(msg)
				continue
			}
			parts := strings.Split(jam, ":")
			if len(parts) != 2 {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Format jam tidak valid!\nContoh: /in 09.30")
				bot.Send(msg)
				continue
			}
			hour, err1 := strconv.Atoi(parts[0])
			minute, err2 := strconv.Atoi(parts[1])
			if err1 != nil || err2 != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Format jam tidak valid!\nContoh: /in 09.30")
				bot.Send(msg)
				continue
			}
			now := time.Now().In(time.FixedZone("WIB", 7*3600))
			inputMinutes := hour*60 + minute
			currentMinutes := now.Hour()*60 + now.Minute()
			if inputMinutes > currentMinutes {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Perhatikan Jam saat ini guys!\n")
				bot.Send(msg)
				continue
			}
			stateMutex.Lock()
			existingJam := state.TransferJam.JamMasuk
			stateMutex.Unlock()
			if existingJam != "" {
				existingParts := strings.Split(existingJam, ":")
				if len(existingParts) == 2 {
					exHour, _ := strconv.Atoi(existingParts[0])
					exMinute, _ := strconv.Atoi(existingParts[1])
					existingMinutes := exHour*60 + exMinute
					if inputMinutes <= existingMinutes {
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, "✅ Terimakasih telah berpartisipasi, ingfo ini sangat bermanfaat bagi orang lain 🙏🏻")
						bot.Send(msg)
						continue
					}
				}
			}
			durationMinutes := currentMinutes - inputMinutes
			if durationMinutes < 0 {
				durationMinutes = 0
			}
			stateMutex.Lock()
			state.TransferJam = TransferJam{
				JamMasuk:   jam,
				Durasi:     formatDuration(durationMinutes),
				LastUpdate: now.Format("15:04"),
			}
			stateMutex.Unlock()
			BroadcastState(GetStateBytes())
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("✅ Jam transfer: %s\nTerimakasih telah berpartisipasi, ingfo ini sangat bermanfaat bagi orang lain 🙏🏻", jam))
			bot.Send(msg)
			continue
		}

		// /atur <teks>
		if strings.HasPrefix(text, "/atur") {
			isi := strings.TrimSpace(strings.TrimPrefix(text, "/atur"))
			status := "🚫"
			if int64(userID) == adminID {
				status = "✅"
			}
			sendLogToAdmin(bot, user, "atur", isi, status)
			if int64(userID) != adminID {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "⛔ Perintah ini hanya untuk Admin.")
				bot.Send(msg)
				continue
			}
			if isi == "" {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Gunakan: /atur <kalimat>")
				bot.Send(msg)
				continue
			}
			stateMutex.Lock()
			state.TreasuryInfo = strings.ReplaceAll(strings.ReplaceAll(isi, "  ", "&nbsp;&nbsp;"), "\n", "<br>")
			stateMutex.Unlock()
			BroadcastState(GetStateBytes())
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "✅ Info Treasury berhasil diubah!")
			bot.Send(msg)
			continue
		}

		// /resetjam
		if strings.HasPrefix(text, "/resetjam") {
			status := "🚫"
			if int64(userID) == adminID {
				status = "✅"
			}
			sendLogToAdmin(bot, user, "resetjam", "", status)
			if int64(userID) != adminID {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "⛔ Perintah ini hanya untuk Admin.")
				bot.Send(msg)
				continue
			}
			stateMutex.Lock()
			state.TransferJam = TransferJam{}
			stateMutex.Unlock()
			BroadcastState(GetStateBytes())
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "✅ Data transfer telah direset")
			bot.Send(msg)
			continue
		}

		// /banid <id>
		if strings.HasPrefix(text, "/banid") {
			idstr := strings.TrimSpace(strings.TrimPrefix(text, "/banid"))
			status := "🚫"
			if int64(userID) == adminID {
				status = "✅"
			}
			sendLogToAdmin(bot, user, "banid", idstr, status)
			if int64(userID) != adminID {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "⛔ Perintah ini hanya untuk Admin.")
				bot.Send(msg)
				continue
			}
			if idstr == "" {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Gunakan: /banid <user_id>\nContoh: /banid 123456789")
				bot.Send(msg)
				continue
			}
			targetID, err := strconv.ParseInt(idstr, 10, 64)
			if err != nil {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "❌ ID harus berupa angka!")
				bot.Send(msg)
				continue
			}
			if targetID == int64(userID) {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Anda tidak bisa ban diri sendiri!")
				bot.Send(msg)
				continue
			}
			if banned[targetID] {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("ℹ️ User ID <code>%d</code> sudah dalam daftar banned.", targetID))
				msg.ParseMode = "HTML"
				bot.Send(msg)
				continue
			}
			banned[targetID] = true
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("✅ <b>User Dibanned</b>\n━━━━━━━━━━━━━━━\n🆔 User ID: <code>%d</code>\n📊 Total banned: %d user", targetID, len(banned)))
			msg.ParseMode = "HTML"
			bot.Send(msg)
			continue
		}

		// /unbanid <id>
		if strings.HasPrefix(text, "/unbanid") {
			idstr := strings.TrimSpace(strings.TrimPrefix(text, "/unbanid"))
			status := "🚫"
			if int64(userID) == adminID {
				status = "✅"
			}
			sendLogToAdmin(bot, user, "unbanid", idstr, status)
			if int64(userID) != adminID {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "⛔ Perintah ini hanya untuk Admin.")
				bot.Send(msg)
				continue
			}
			if idstr == "" {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Gunakan: /unbanid <user_id>\nContoh: /unbanid 123456789")
				bot.Send(msg)
				continue
			}
			targetID, err := strconv.ParseInt(idstr, 10, 64)
			if err != nil {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "❌ ID harus berupa angka!")
				bot.Send(msg)
				continue
			}
			if !banned[targetID] {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("ℹ️ User ID <code>%d</code> tidak ada dalam daftar banned.", targetID))
				msg.ParseMode = "HTML"
				bot.Send(msg)
				continue
			}
			delete(banned, targetID)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("✅ <b>User Diunban</b>\n━━━━━━━━━━━━━━━\n🆔 User ID: <code>%d</code>\n📊 Total banned: %d user", targetID, len(banned)))
			msg.ParseMode = "HTML"
			bot.Send(msg)
			continue
		}

		// /listban
		if strings.HasPrefix(text, "/listban") {
			status := "🚫"
			if int64(userID) == adminID {
				status = "✅"
			}
			sendLogToAdmin(bot, user, "listban", "", status)
			if int64(userID) != adminID {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "⛔ Perintah ini hanya untuk Admin.")
				bot.Send(msg)
				continue
			}
			if len(banned) == 0 {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "📋 Tidak ada user yang dibanned.")
				bot.Send(msg)
				continue
			}
			var ids []string
			for k := range banned {
				ids = append(ids, fmt.Sprintf("• <code>%d</code>", k))
			}
			sort.Strings(ids)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("📋 <b>Daftar User Banned</b>\n━━━━━━━━━━━━━━━━━━━\n%s\n\n📊 Total: %d user", strings.Join(ids, "\n"), len(banned)))
			msg.ParseMode = "HTML"
			bot.Send(msg)
			continue
		}
	}
}
