package dot

import (
	"bytes"
	"fmt"
	SpringCore "github.com/go-spring/spring-core"
	"testing"
)

func TestDotWithContext(t *testing.T) {
	applicationContext := SpringCore.DefaultApplicationContext()
	applicationContext.RegisterBean(&IamTestHandler{})
	applicationContext.RegisterBean(&IamTestSimpleController{})
	applicationContext.RegisterBean(&IamTestComplexController{})

	var validators = []IamTestValidator{
		&IamTest0Validator{},
		&IamTest1Validator{},
		&IamTest2Validator{},
		&IamTest3Validator{},
		&IamTest4Validator{},
		&IamTest5Validator{},
	}

	applicationContext.RegisterNameBeanFn("iamTestValidator", func() IamTestValidator {
		return validators[0]
	})

	applicationContext.RegisterBean(validators)

	var services = []IamTestService{
		&IamTestServiceRemixImpl{},
		&IamTestServiceDBImpl{},
		&IamTestServiceRedisImpl{},
	}
	applicationContext.RegisterNameBeanFn("iamTestService", func() IamTestService {
		return services[0]
	})

	applicationContext.RegisterBean(services)

	applicationContext.RegisterBean(&Redis{})
	applicationContext.RegisterBean(&Datasource{})

	applicationContext.RegisterNameBean("handler-map", map[string]interface{}{
		"a":111,
		"b":"ccc",
	})

	applicationContext.AutoWireBeans()
	buffer := bytes.Buffer{}
	_ = WithContext(applicationContext).WriteDot(&buffer)
	fmt.Println(buffer.String())
}


// 测试用结构体
type (
	IamTestHandler struct {
		IamTestSimpleController  *IamTestSimpleController  `autowire:""`
		IamTestComplexController *IamTestComplexController `autowire:""`
	}
	IamTestValidator interface {
		Validate() error
	}
	IamTestService interface {
		Test() bool
	}
)

type (
	IamTest0Validator struct {
	}
	IamTest1Validator struct {
	}
	IamTest2Validator struct {
	}
	IamTest3Validator struct {
	}
	IamTest4Validator struct {
	}
	IamTest5Validator struct {
	}
)

type (
	IamTestSimpleController struct {
		Validator IamTestValidator `autowire:"iamTestValidator"`
		Service   IamTestService   `autowire:"iamTestService"`
	}
	IamTestComplexController struct {
		Validators []IamTestValidator `autowire:""`
		Services   []IamTestService   `autowire:""`
	}
	IamTestServiceDBImpl struct {
		Datasource *Datasource `autowire:""`
	}
	Datasource struct {
		Host     string
		Port     uint16
		Protocol string
	}
	IamTestServiceRedisImpl struct {
		Redis *Redis `autowire:""`
	}
	Redis struct {
		Host string
		Port uint16
	}
	IamTestServiceRemixImpl struct {
		Datasource *Datasource `autowire:""`
		Redis      *Redis      `autowire:""`
	}
)


func (validator *IamTest0Validator) Validate() error {
	return nil
}

func (validator *IamTest1Validator) Validate() error {
	return nil
}

func (validator *IamTest2Validator) Validate() error {
	return nil
}

func (validator *IamTest3Validator) Validate() error {
	return nil
}

func (validator *IamTest4Validator) Validate() error {
	return nil
}

func (validator *IamTest5Validator) Validate() error {
	return nil
}

func (service *IamTestServiceDBImpl) Test() bool {
	return true
}

func (service *IamTestServiceRedisImpl) Test() bool {
	return true
}

func (service *IamTestServiceRemixImpl) Test() bool {
	return true
}