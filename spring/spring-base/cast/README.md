# cast

Provides many conversion functions between types.

## Install

```
go get github.com/go-spring/spring-base@v1.1.0-rc2 
```

## Import

```
import "github.com/go-spring/spring-base/cast"
```

## Example

```
fmt.Println(cast.ToInt(10))   // 10
fmt.Println(cast.ToInt(10.0)) // 10
fmt.Println(cast.ToInt("10")) // 10
fmt.Println(cast.ToInt(true)) // 1
```