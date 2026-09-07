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

package luohua

import (
	"go-spring.org/cloud/i18n"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
)

// luohuaCatalog builds luohua's error catalog — one MessageSource carrying the
// company's message keys in the languages it serves. A company extends this in
// one place (here) rather than per component; the catalog is what error /
// validation rendering localizes against.
func luohuaCatalog(defaultLocale string) *i18n.MapSource {
	return i18n.NewMapSource(i18n.WithDefaultLocale(defaultLocale)).
		AddMessage("zh", "luohua.tenant.missing", "缺少租户标识 {0}").
		AddMessage("zh", "luohua.orders.denied", "无权访问订单资源").
		AddMessage("en", "luohua.tenant.missing", "tenant {0} is missing").
		AddMessage("en", "luohua.orders.denied", "not authorized for orders").
		AddBundle("zh", map[string]string{
			"validation.required": "{0} 不能为空",
			"validation.min":      "{0} 不得小于 {1}",
		}).
		AddBundle("en", map[string]string{
			"validation.required": "{0} is required",
			"validation.min":      "{0} must be at least {1}",
		})
}

func init() {
	// Armed by any spring.luohua.i18n.* key. The catalog is a default: if the
	// application registers its own i18n.MessageSource, OnMissingBean steps
	// luohua's aside rather than competing.
	gs.Module(gs.OnProperty("spring.luohua.i18n"), func(r gs.BeanProvider, p flatten.Storage) error {
		if off, err := disabled(p); err != nil {
			return err
		} else if off {
			return nil // whole baseline off; do not assemble the keyed bean capability
		}
		var c I18nConfig
		if err := conf.Bind(p, &c, "${spring.luohua.i18n:=}"); err != nil {
			return err
		}
		r.Provide(func() *i18n.MapSource { return luohuaCatalog(c.DefaultLocale) }).
			Condition(gs.OnMissingBean[i18n.MessageSource]()).
			Export(gs.As[i18n.MessageSource]()).Caller(1)
		return nil
	})
}
