/*
 * Copyright 2012-2019 the original author or authors.
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

package i18n

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-spring/spring-base/code"
	"github.com/go-spring/spring-base/knife"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/conf"
)

// Languages 语言缩写代码表。
var _ = []string{
	"af",     //南非语
	"af-ZA",  //南非语
	"ar",     //阿拉伯语
	"ar-AE",  //阿拉伯语(阿联酋)
	"ar-BH",  //阿拉伯语(巴林)
	"ar-DZ",  //阿拉伯语(阿尔及利亚)
	"ar-EG",  //阿拉伯语(埃及)
	"ar-IQ",  //阿拉伯语(伊拉克)
	"ar-JO",  //阿拉伯语(约旦)
	"ar-KW",  //阿拉伯语(科威特)
	"ar-LB",  //阿拉伯语(黎巴嫩)
	"ar-LY",  //阿拉伯语(利比亚)
	"ar-MA",  //阿拉伯语(摩洛哥)
	"ar-OM",  //阿拉伯语(阿曼)
	"ar-QA",  //阿拉伯语(卡塔尔)
	"ar-SA",  //阿拉伯语(沙特阿拉伯)
	"ar-SY",  //阿拉伯语(叙利亚)
	"ar-TN",  //阿拉伯语(突尼斯)
	"ar-YE",  //阿拉伯语(也门)
	"az",     //阿塞拜疆语
	"az-AZ",  //阿塞拜疆语(拉丁文)
	"az-AZ",  //阿塞拜疆语(西里尔文)
	"be",     //比利时语
	"be-BY",  //比利时语
	"bg",     //保加利亚语
	"bg-BG",  //保加利亚语
	"bs-BA",  //波斯尼亚语(拉丁文，波斯尼亚和黑塞哥维那)
	"ca",     //加泰隆语
	"ca-ES",  //加泰隆语
	"cs",     //捷克语
	"cs-CZ",  //捷克语
	"cy",     //威尔士语
	"cy-GB",  //威尔士语
	"da",     //丹麦语
	"da-DK",  //丹麦语
	"de",     //德语
	"de-AT",  //德语(奥地利)
	"de-CH",  //德语(瑞士)
	"de-DE",  //德语(德国)
	"de-LI",  //德语(列支敦士登)
	"de-LU",  //德语(卢森堡)
	"dv",     //第维埃语
	"dv-MV",  //第维埃语
	"el",     //希腊语
	"el-GR",  //希腊语
	"en",     //英语
	"en-AU",  //英语(澳大利亚)
	"en-BZ",  //英语(伯利兹)
	"en-CA",  //英语(加拿大)
	"en-CB",  //英语(加勒比海)
	"en-GB",  //英语(英国)
	"en-IE",  //英语(爱尔兰)
	"en-JM",  //英语(牙买加)
	"en-NZ",  //英语(新西兰)
	"en-PH",  //英语(菲律宾)
	"en-TT",  //英语(特立尼达)
	"en-US",  //英语(美国)
	"en-ZA",  //英语(南非)
	"en-ZW",  //英语(津巴布韦)
	"eo",     //世界语
	"es",     //西班牙语
	"es-AR",  //西班牙语(阿根廷)
	"es-BO",  //西班牙语(玻利维亚)
	"es-CL",  //西班牙语(智利)
	"es-CO",  //西班牙语(哥伦比亚)
	"es-CR",  //西班牙语(哥斯达黎加)
	"es-DO",  //西班牙语(多米尼加共和国)
	"es-EC",  //西班牙语(厄瓜多尔)
	"es-ES",  //西班牙语(传统)
	"es-ES",  //西班牙语(国际)
	"es-GT",  //西班牙语(危地马拉)
	"es-HN",  //西班牙语(洪都拉斯)
	"es-MX",  //西班牙语(墨西哥)
	"es-NI",  //西班牙语(尼加拉瓜)
	"es-PA",  //西班牙语(巴拿马)
	"es-PE",  //西班牙语(秘鲁)
	"es-PR",  //西班牙语(波多黎各(美))
	"es-PY",  //西班牙语(巴拉圭)
	"es-SV",  //西班牙语(萨尔瓦多)
	"es-UY",  //西班牙语(乌拉圭)
	"es-VE",  //西班牙语(委内瑞拉)
	"et",     //爱沙尼亚语
	"et-EE",  //爱沙尼亚语
	"eu",     //巴士克语
	"eu-ES",  //巴士克语
	"fa",     //法斯语
	"fa-IR",  //法斯语
	"fi",     //芬兰语
	"fi-FI",  //芬兰语
	"fo",     //法罗语
	"fo-FO",  //法罗语
	"fr",     //法语
	"fr-BE",  //法语(比利时)
	"fr-CA",  //法语(加拿大)
	"fr-CH",  //法语(瑞士)
	"fr-FR",  //法语(法国)
	"fr-LU",  //法语(卢森堡)
	"fr-MC",  //法语(摩纳哥)
	"gl",     //加里西亚语
	"gl-ES",  //加里西亚语
	"gu",     //古吉拉特语
	"gu-IN",  //古吉拉特语
	"he",     //希伯来语
	"he-IL",  //希伯来语
	"hi",     //印地语
	"hi-IN",  //印地语
	"hr",     //克罗地亚语
	"hr-BA",  //克罗地亚语(波斯尼亚和黑塞哥维那)
	"hr-HR",  //克罗地亚语
	"hu",     //匈牙利语
	"hu-HU",  //匈牙利语
	"hy",     //亚美尼亚语
	"hy-AM",  //亚美尼亚语
	"id",     //印度尼西亚语
	"id-ID",  //印度尼西亚语
	"is",     //冰岛语
	"is-IS",  //冰岛语
	"it",     //意大利语
	"it-CH",  //意大利语(瑞士)
	"it-IT",  //意大利语(意大利)
	"ja",     //日语
	"ja-JP",  //日语
	"ka",     //格鲁吉亚语
	"ka-GE",  //格鲁吉亚语
	"kk",     //哈萨克语
	"kk-KZ",  //哈萨克语
	"kn",     //卡纳拉语
	"kn-IN",  //卡纳拉语
	"ko",     //朝鲜语
	"ko-KR",  //朝鲜语
	"kok",    //孔卡尼语
	"kok-IN", //孔卡尼语
	"ky",     //吉尔吉斯语
	"ky-KG",  //吉尔吉斯语(西里尔文)
	"lt",     //立陶宛语
	"lt-LT",  //立陶宛语
	"lv",     //拉脱维亚语
	"lv-LV",  //拉脱维亚语
	"mi",     //毛利语
	"mi-NZ",  //毛利语
	"mk",     //马其顿语
	"mk-MK",  //马其顿语(FYROM)
	"mn",     //蒙古语
	"mn-MN",  //蒙古语(西里尔文)
	"mr",     //马拉地语
	"mr-IN",  //马拉地语
	"ms",     //马来语
	"ms-BN",  //马来语(文莱达鲁萨兰)
	"ms-MY",  //马来语(马来西亚)
	"mt",     //马耳他语
	"mt-MT",  //马耳他语
	"nb",     //挪威语(伯克梅尔)
	"nb-NO",  //挪威语(伯克梅尔)(挪威)
	"nl",     //荷兰语
	"nl-BE",  //荷兰语(比利时)
	"nl-NL",  //荷兰语(荷兰)
	"nn-NO",  //挪威语(尼诺斯克)(挪威)
	"ns",     //北梭托语
	"ns-ZA",  //北梭托语
	"pa",     //旁遮普语
	"pa-IN",  //旁遮普语
	"pl",     //波兰语
	"pl-PL",  //波兰语
	"pt",     //葡萄牙语
	"pt-BR",  //葡萄牙语(巴西)
	"pt-PT",  //葡萄牙语(葡萄牙)
	"qu",     //克丘亚语
	"qu-BO",  //克丘亚语(玻利维亚)
	"qu-EC",  //克丘亚语(厄瓜多尔)
	"qu-PE",  //克丘亚语(秘鲁)
	"ro",     //罗马尼亚语
	"ro-RO",  //罗马尼亚语
	"ru",     //俄语
	"ru-RU",  //俄语
	"sa",     //梵文
	"sa-IN",  //梵文
	"se",     //北萨摩斯语
	"se-FI",  //北萨摩斯语(芬兰)
	"se-FI",  //斯科特萨摩斯语(芬兰)
	"se-FI",  //伊那里萨摩斯语(芬兰)
	"se-NO",  //北萨摩斯语(挪威)
	"se-NO",  //律勒欧萨摩斯语(挪威)
	"se-NO",  //南萨摩斯语(挪威)
	"se-SE",  //北萨摩斯语(瑞典)
	"se-SE",  //律勒欧萨摩斯语(瑞典)
	"se-SE",  //南萨摩斯语(瑞典)
	"sk",     //斯洛伐克语
	"sk-SK",  //斯洛伐克语
	"sl",     //斯洛文尼亚语
	"sl-SI",  //斯洛文尼亚语
	"sq",     //阿尔巴尼亚语
	"sq-AL",  //阿尔巴尼亚语
	"sr-BA",  //塞尔维亚语(拉丁文，波斯尼亚和黑塞哥维那)
	"sr-BA",  //塞尔维亚语(西里尔文，波斯尼亚和黑塞哥维那)
	"sr-SP",  //塞尔维亚(拉丁)
	"sr-SP",  //塞尔维亚(西里尔文)
	"sv",     //瑞典语
	"sv-FI",  //瑞典语(芬兰)
	"sv-SE",  //瑞典语
	"sw",     //斯瓦希里语
	"sw-KE",  //斯瓦希里语
	"syr",    //叙利亚语
	"syr-SY", //叙利亚语
	"ta",     //泰米尔语
	"ta-IN",  //泰米尔语
	"te",     //泰卢固语
	"te-IN",  //泰卢固语
	"th",     //泰语
	"th-TH",  //泰语
	"tl",     //塔加路语
	"tl-PH",  //塔加路语(菲律宾)
	"tn",     //茨瓦纳语
	"tn-ZA",  //茨瓦纳语
	"tr",     //土耳其语
	"tr-TR",  //土耳其语
	"ts",     //宗加语
	"tt",     //鞑靼语
	"tt-RU",  //鞑靼语
	"uk",     //乌克兰语
	"uk-UA",  //乌克兰语
	"ur",     //乌都语
	"ur-PK",  //乌都语
	"uz",     //乌兹别克语
	"uz-UZ",  //乌兹别克语(拉丁文)
	"uz-UZ",  //乌兹别克语(西里尔文)
	"vi",     //越南语
	"vi-VN",  //越南语
	"xh",     //班图语
	"xh-ZA",  //班图语
	"zh",     //中文
	"zh-CN",  //中文(简体)
	"zh-HK",  //中文(香港)
	"zh-MO",  //中文(澳门)
	"zh-SG",  //中文(新加坡)
	"zh-TW",  //中文(繁体)
	"zu",     //祖鲁语
	"zu-ZA",  //祖鲁语
}

// defaultLanguage 设置默认语言。
var defaultLanguage = "zh-CN"

// languageMap 语言配置表。
var languageMap = make(map[string]*conf.Properties)

// Register 注册语言配置表。
func Register(language string, data *conf.Properties) error {
	if _, ok := languageMap[language]; ok {
		return errors.New("duplicate language")
	}
	languageMap[language] = data
	return nil
}

// LoadLanguage 加载语言文件。
func LoadLanguage(filename string) error {
	fileInfo, err := os.Stat(filename)
	if err != nil {
		return err
	}
	if fileInfo.IsDir() {
		return loadLanguageFromDir(filename)
	}
	return loadLanguageFromFile(filename)
}

func loadLanguageFromDir(dir string) error {
	dirNames, err := util.ReadDirNames(dir)
	if err != nil {
		return err
	}
	p := conf.New()
	var fileInfo os.FileInfo
	for _, name := range dirNames {
		filename := filepath.Join(dir, name)
		fileInfo, err = os.Stat(filename)
		if err != nil {
			return err
		}
		if fileInfo.IsDir() {
			continue
		}
		if err = p.Load(filename); err != nil {
			return err
		}
	}
	language := filepath.Base(dir)
	return Register(language, p)
}

func loadLanguageFromFile(file string) error {
	p, err := conf.Load(file)
	if err != nil {
		return err
	}
	filename := filepath.Base(file)
	language := strings.Split(filename, ".")[0]
	return Register(language, p)
}

const languageKey = "::language::"

// SetDefaultLanguage 设置默认语言。
func SetDefaultLanguage(language string) {
	defaultLanguage = language
}

// SetLanguage 设置上下文语言。
func SetLanguage(ctx context.Context, language string) error {
	return knife.Store(ctx, languageKey, language)
}

// Get 获取语言对应的配置项，从 context.Context 中获取上下文语言。
func Get(ctx context.Context, key string) string {

	language := defaultLanguage
	v, err := knife.Load(ctx, languageKey)
	if err == nil {
		str, ok := v.(string)
		if ok {
			language = str
		}
	}

	if m, ok := languageMap[language]; ok && m != nil {
		if m.Has(key) {
			return m.Get(key)
		}
	}

	ss := strings.SplitN(language, "-", 2)
	if len(ss) < 2 {
		return ""
	}

	if m, ok := languageMap[ss[0]]; ok && m != nil {
		return m.Get(key)
	}
	return ""
}

// Resolve 获取语言对应的配置项，从 context.Context 中获取上下文语言。
func Resolve(ctx context.Context, s string) (string, error) {
	return resolveString(ctx, s)
}

func resolveString(ctx context.Context, s string) (string, error) {

	n := len(s)
	count := 0
	found := false
	start, end := -1, -1

	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			if i < n-1 {
				if s[i+1] == '{' {
					if count == 0 {
						start = i
					}
					count++
				}
			}
		case '}':
			if i < n-1 {
				if s[i+1] == '}' {
					count--
					if count == 0 {
						found = true
						i++
						end = i
					}
				}
			}
		}
		if found {
			break
		}
	}

	if start < 0 || end < 0 {
		return s, nil
	}

	if count > 0 {
		return "", util.Errorf(code.FileLine(), "%s 语法错误", s)
	}

	key := strings.TrimSpace(s[start+2 : end-1])
	s1 := Get(ctx, key)

	s2, err := resolveString(ctx, s[end+1:])
	if err != nil {
		return "", err
	}

	return s[:start] + s1 + s2, nil
}
