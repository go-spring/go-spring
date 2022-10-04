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

package color_test

import (
	"fmt"
	"testing"

	"github.com/go-spring/spring-base/color"
)

func TestColor(t *testing.T) {

	fmt.Println(color.Bold.Sprint("ok"))
	fmt.Println(color.Italic.Sprint("ok"))
	fmt.Println(color.Underline.Sprint("ok"))
	fmt.Println(color.ReverseVideo.Sprint("ok"))
	fmt.Println(color.CrossedOut.Sprint("ok"))

	fmt.Println(color.Black.Sprintf("ok"))
	fmt.Println(color.Red.Sprintf("ok"))
	fmt.Println(color.Green.Sprintf("ok"))
	fmt.Println(color.Yellow.Sprintf("ok"))
	fmt.Println(color.Blue.Sprintf("ok"))
	fmt.Println(color.Magenta.Sprintf("ok"))
	fmt.Println(color.Cyan.Sprintf("ok"))
	fmt.Println(color.White.Sprintf("ok"))

	fmt.Println(color.BgBlack.Sprint("ok"))
	fmt.Println(color.BgRed.Sprint("ok"))
	fmt.Println(color.BgGreen.Sprint("ok"))
	fmt.Println(color.BgYellow.Sprint("ok"))
	fmt.Println(color.BgBlue.Sprint("ok"))
	fmt.Println(color.BgMagenta.Sprint("ok"))
	fmt.Println(color.BgCyan.Sprint("ok"))
	fmt.Println(color.BgWhite.Sprint("ok"))

	attributes := []color.Attribute{
		color.Bold,
		color.Italic,
		color.Underline,
		color.ReverseVideo,
		color.CrossedOut,
		color.Red,
		color.BgGreen,
	}

	fmt.Println(color.NewText().Sprint("ok"))
	fmt.Println(color.NewText(attributes...).Sprint("ok"))
	fmt.Println(color.NewText(attributes...).Sprintf("ok"))
}
