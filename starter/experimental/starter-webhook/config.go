/*
 * Copyright 2025 The Go-Spring Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package StarterWebhook

import (
	"time"

)

// Config defines one webhook notifier instance.
type Config struct {
	// URL is the webhook endpoint the notification is POSTed to.
	URL string `value:"${url}" expr:"$ != ''"`

	// Channel selects the payload format (and signing scheme) of the
	// receiver: "generic" (plain JSON POST), "dingtalk", "feishu", "wecom" or
	// "slack". Default is "generic".
	Channel string `value:"${channel:=generic}"`

	// Secret enables signing where the channel supports it: the DingTalk
	// "加签" secret (SEC...) or the Feishu signing secret. Ignored by the
	// other channels.
	Secret string `value:"${secret:=}"`

	// Timeout bounds one POST, e.g., "5s".
	Timeout time.Duration `value:"${timeout:=5s}"`
}
