# color

提供了一些控制台输出格式。

## Install

```
go get github.com/go-spring/spring-base@v1.1.0-rc2 
```

## Import

```
import "github.com/go-spring/spring-base/color"
```

## Example

```
fmt.Println(color.BgBlack.Sprint("ok"))
fmt.Println(color.BgRed.Sprint("ok"))
fmt.Println(color.BgGreen.Sprint("ok"))
fmt.Println(color.BgYellow.Sprint("ok"))

attributes := []color.Attribute{
    color.Bold,
    color.Italic,
    color.Underline,
    color.ReverseVideo,
    color.CrossedOut,
    color.Red,
    color.BgGreen,
}

fmt.Println(color.NewText(attributes...).Sprint("ok"))
```