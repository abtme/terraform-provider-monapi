resource "monapi_channel" "ops" {
  name       = "Ops email"
  kind       = "email"
  to         = ["ops@example.com"]
  all_checks = false
}

resource "monapi_channel" "telegram" {
  name      = "Telegram"
  kind      = "telegram"
  bot_token = var.telegram_bot_token
  chat_id   = "-1001234567890"
}

variable "telegram_bot_token" {
  type      = string
  sensitive = true
}
