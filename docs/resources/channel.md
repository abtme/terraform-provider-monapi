---
page_title: "monapi_channel Resource - monapi"
subcategory: ""
description: |-
  A notification channel: where state changes are sent. Needs an owner token. The server keeps the settings sealed and never returns them, so changes made outside Terraform to to, bot_token, chat_id, webhook_url or url are not seen; an import cannot fill them either (set them in the configuration and apply).
---

# monapi_channel (Resource)

A notification channel: where state changes are sent. Needs an owner token. The server keeps the settings sealed and never returns them, so changes made outside Terraform to to, bot_token, chat_id, webhook_url or url are not seen; an import cannot fill them either (set them in the configuration and apply).

## Example Usage

```terraform
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
```

## Schema

### Required

- `kind` (String) email, telegram, slack, teams or webhook. Changing it makes a new channel.
- `name` (String)

### Optional

- `all_checks` (Boolean) Send for every check (default). If false, only for checks that list this channel in channel_ids.
- `bot_token` (String) telegram: the bot's token from @BotFather. (sensitive)
- `chat_id` (String) telegram: a chat or channel id (-100...) or @channelname.
- `enabled` (Boolean)
- `to` (List of String) email: up to 10 addresses.
- `url` (String) webhook: where to POST (https).
- `webhook_url` (String) slack: an incoming webhook URL; teams: a Workflows "post to a channel" URL. (sensitive)

### Read-Only

- `id` (String)
- `summary` (String) What the server shows for the settings (secrets hidden).
- `webhook_secret` (String) webhook: the key the X-Monapi-Signature HMAC is made with. (sensitive)

## Import

Import is supported using the following syntax:

```shell
terraform import monapi_channel.example <channel_id>
```
