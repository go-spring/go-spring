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

package console_test

import (
	"fmt"
	"testing"

	"github.com/go-spring/spring-boost/console"
)

func TestText(t *testing.T) {

	fmt.Println(console.Bold.Sprint("ok"))
	fmt.Println(console.Italic.Sprint("ok"))
	fmt.Println(console.Underline.Sprint("ok"))
	fmt.Println(console.ReverseVideo.Sprint("ok"))
	fmt.Println(console.CrossedOut.Sprint("ok"))

	fmt.Println(console.Black.Sprint("ok"))
	fmt.Println(console.Red.Sprint("ok"))
	fmt.Println(console.Green.Sprint("ok"))
	fmt.Println(console.Yellow.Sprint("ok"))
	fmt.Println(console.Blue.Sprint("ok"))
	fmt.Println(console.Magenta.Sprint("ok"))
	fmt.Println(console.Cyan.Sprint("ok"))
	fmt.Println(console.White.Sprint("ok"))

	fmt.Println(console.BgBlack.Sprint("ok"))
	fmt.Println(console.BgRed.Sprint("ok"))
	fmt.Println(console.BgGreen.Sprint("ok"))
	fmt.Println(console.BgYellow.Sprint("ok"))
	fmt.Println(console.BgBlue.Sprint("ok"))
	fmt.Println(console.BgMagenta.Sprint("ok"))
	fmt.Println(console.BgCyan.Sprint("ok"))
	fmt.Println(console.BgWhite.Sprint("ok"))

	attributes := []console.Attribute{
		console.Bold,
		console.Italic,
		console.Underline,
		console.ReverseVideo,
		console.CrossedOut,
		console.Red,
		console.BgGreen,
	}

	fmt.Println(console.NewText(attributes...).Sprint("ok"))
}
