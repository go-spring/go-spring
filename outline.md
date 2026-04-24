# 一、配置

1. 配置核心
    1. 配置数据模型
        1. key-value 配置模型
        2. 层级配置结构（树形）
        3. 配置扁平化
            1. path 语法
                1. a.b.c
                2. a.b[0].c
        4. 配置项元信息
            1. key
            2. value
            3. 来源（source）
    2. 配置加载
        1. 配置格式
            1. 读取不同格式的配置文件
            2. 根据文件后缀选择解析器
                1. properties
                2. yaml/yml
                3. toml/tml
                4. json
            3. 支持注册自定义格式解析器
        2. 配置来源（provider）
            1. 读取不同来源的配置文件
            2. 根据来源选择读取器
                1. file
                2. k8s-config
                3. etcd
                4. nacos
                5. zookeeper
            3. 支持注册自定义配置来源
            4. 初始阶段不使用 ioc，只能极简实现
            5. 运行期动态刷新，可以集成进 ioc 体系
    3. 配置表达与解析
        1. 表达式语法
            1. ${key}
            2. ${key:=value}
        2. 字符串分割器
            1. \>>splitter
        3. resolving
            1. 自动计算配置依赖
            2. 递归 resolving
    4. 配置绑定
        1. 绑定到 Go 结构体
            1. 基础类型
            2. slice/map
            3. 嵌套字段
            4. 嵌套结构体
            5. 自定义类型
        2. 类型转换器
            1. 内置类型转换器
                1. time.Time
                2. time.Duration
            2. 自定义类型转换器
        3. 字符串分割器
            1. 内置字符串分割器
                1. 按照英文逗号进行分割
            2. 自定义字符串分割器
    5. 配置校验
        1. 必填字段
            1. 不填视为失败
            2. 除非设置默认值
        2. 表达式校验
            1. 数值范围
            2. 枚举
            3. 正则
        3. 自定义校验函数
        4. $ 特殊字符，表示当前字段的值
2. 应用配置
    1. Profile 与环境
        1. profile 激活
            1. spring.profiles.active
        2. profile 确定时机
            1. 命令行参数
            2. 环境变量
    2. 配置合并与优先级
        1. 固定配置优先级，从大到小
            1. 命令行参数
            2. 环境变量
            3. 文件配置（包含import）
            4. 代码配置
        2. 同名 key 覆盖规则
            1. 高层级覆盖低层级
    3. 命令行参数
        1. 参数筛选
            1. -D 前缀
    4. 环境变量
        1. 参数筛选
            1. GS_ 前缀
    5. 配置导入（Import）
        1. 借鉴 spring 配置文件的 import 机制
            1. spring.app.imports
        2. 支持来源
            1. 本地文件
            2. 远程配置
        3. optional 语义
            1. optional:provider:xxx
    6. 代码配置
    7. 配置结果追踪
        1. 合并结果 dump
        2. 配置项来源可视化
3. 动态配置
    1. 动态配置模型
        1. 使用与静态配置一致的语法
        2. 动态字段声明
            1. gs.Dync 泛型
    2. 配置变更机制
        1. 原子变更
        2. 版本控制
        3. 事件机制
            1. listener
    3. 动态刷新流程
        1. 注入刷新句柄
            1. ConfigRefresher 对象
        2. 监听刷新信号
        3. 合并新配置
        4. 预刷新
            1. 失败不提交
        5. 按需刷新
            1. 内容不变不刷新
            2. 静态字段不刷新

# 二、日志

1. 日志模型
    1. 架构模型
        1. 遵循 log4j2 的经典架构模型
        2. 非树形结构
            1. 不基于 logger name 的层级继承
            2. 通过 tag 打破 logger name 的继承规则
        3. tag 路由规则
            1. tag 精确或前缀匹配到 logger
            2. 未匹配到任何 logger 的 tag，日志输出到 root logger
        4. 默认存在 root logger
        5. 其他 logger 由用户显式配置
    2. Logger
        1. 对接统一日志 API，作为日志路由与分流层
        2. logger 本身不负责存储，仅负责将日志分发给 appender
        3. 简单 Logger（Simple Logger）
            1. 必须与至少一个 appender 搭配使用
            2. 仅作为日志通道与分发器，不提供额外语义
            3. 同步 Logger
                1. 同步模型
                2. 日志顺序发送给所有 appender
            4. 异步 Logger（Async Logger）
                1. 异步模型，内部维护 buffer
                2. 日志先写入队列，由后端 appender 消费
                3. 队列满时的处理策略：丢弃、阻塞
        4. 集成 Logger（Composite Logger）
            1. 对简单 logger 的预制化封装
            2. 提供开箱即用的常见 logger 类型
                1. console logger
                    1. 输出到控制台
                    2. 内置一个 console appender
                2. file logger
                    1. 同步模型，输出到文件
                    2. 不提供文件轮转等高级能力
                    3. 内置一个 file appender
                3. rolling file logger
                    1. 输出到文件
                    2. 支持文件轮转
                    3. 支持 wf 文件独立存储
                    4. 支持同步 / 异步模型
                    5. 内置一个或两个 rolling file appender
                4. discard logger
                    1. 丢弃所有日志，不进行任何输出
        5. 自定义 Logger
            1. 用户可以注册新的 logger 实现类型
            2. 必须显式声明并配置 appender
        6. Appender 约束
            1. 每个 logger 至少包含一个 appender
            2. 多个 appender 按配置顺序依次执行
            3. appender 自行处理写入错误
                1. logger 不感知 appender 的错误状态
    3. Appender
        1. 日志最终存储层
        2. 可对接多种存储系统
        3. 内置 Appender
            1. console appender：输出到控制台
            2. file appender：输出到文件（无高级能力）
            3. rolling file appender：输出到文件，支持轮转
            4. discard appender：忽略日志
        4. 自定义 Appender
            1. 用户可以注册新的 appender 实现类型
        5. 写入失败处理策略
            1. 仅进行指标上报，用于监控与告警
            2. 不记录错误日志，避免递归写入或阻塞风险
    4. Layout
        1. 负责日志内容格式化
        2. 内置 Layout
            1. text layout
                1. 文本格式
                2. 默认使用 || 作为字段分隔符
            2. json layout
                1. JSON 格式输出
        3. 扩展能力
            1. 支持自定义 layout
            2. layout 依赖 encoder 实现结构化输出
    5. AppenderRef
        1. logger 与 appender 的连接组件
        2. 用于描述 logger 使用哪些 appender
    6. Level
        1. none，0，关闭日志
        2. trace，100
        3. debug，200
        4. info，300
        5. warn，400
        6. error，500
        7. panic，600，仅表示日志级别，不触发 panic
        8. fatal，700，仅表示日志级别，不触发 fatal
        9. max，999
        10. 支持自定义 level
    7. Tag（核心创新点）
        1. go-spring logger 的核心创新能力
        2. 通过 tag 进行日志路由，实现统一日志 API
            1. func Record(ctx context.Context, level Level, tag *Tag, skip int, fields ...Field)
        3. 路由规则
            1. 精确匹配优先
            2. 前缀匹配次之
        4. 与 logger name 的区别
            1. tag 可全局定义，按业务语义建模，可定义在三方包中
            2. logger name 通常基于代码位置，缺乏业务语义
            3. 解决基于 logger name 路由粒度过粗的问题
    8. 可观测性支持
        1. 从 context.Context 中提取可观测信息并写入日志
        2. StringFromContext、FieldsFromContext
        3. field key 冲突不做检查，由用户自行保证
2. 结构化日志
    1. 提供完整的结构化日志支持
    2. 所有日志内容以 Field 形式表达
    3. Field
        1. Msg / Msgf（key = msg）
        2. Nil
        3. Bool / BoolPtr / Bools
        4. Int / IntPtr / Ints
        5. Uint / UintPtr / Uints
        6. Float / FloatPtr / Floats
        7. String / StringPtr / Strings
        8. Array / Object / FieldsFromMap
        9. Any / Reflect
        10. 自定义 Field
    4. Encoder
        1. 将 Fields 编码为最终输出格式
        2. 内置支持：
            1. JSON 编码
            2. Text 编码
        3. 支持自定义 encoder
3. 日志配置
    1. KV 配置模型
        1. logger.myLogger.type=console
            1. 定义一个名为 myLogger 的 console logger
        2. 精简配置语言
            1. appender.console!=Console{layout=TextLayout{}}
            2. 为名为 console 的 appender 配置参数
            3. 支持单行完成原本多行配置，是否使用由用户自行选择
    2. 插件化设计
        1. 基于依赖注入模型
        2. 支持注入 element 和 property 注入：
            1. element（如为 appender 注入 layout）
            2. property（如为 async logger 注入 bufferSize）
        3. 支持插件自注册
        4. 支持自定义 property
        5. 支持自定义类型转换
    3. 生命周期管理
        1. start
        2. stop
            1. 等待异步日志完全刷新
    4. 启动期校验
        1. 配置校验失败则启动失败
        2. 启动期可 dump 最终配置结果
    5. 热更新
        1. 当前版本暂不支持
4. 与其他日志库适配
    1. 提供获取指定名称 logger 的能力
    2. 用于兼容三方日志库或迁移场景

# 三、IoC 容器

1. 设计原则
    1. 容器级单例
    2. 启动期 IoC
        1. 不支持运行时 Bean 获取/注入
    3. 条件互斥优于覆盖
    4. 显式优于隐式
        1. 不自动推导接口
        2. 不自动选择 Primary
    5. Go 语义优先
        1. 非 Spring 兼容模型
2. 核心模型
    1. Bean
        1. 基本定义
            1. 只能是 single 单例
                1. 容器级唯一
            2. Bean ID
                1. Bean 在 IoC 容器中的唯一身份
                2. 强类型+名字(可选)
            3. Bean 来源
                1. 全局注册
                2. Module 注册
                3. 容器注册
                4. Configuration 导出 (子bean)
        2. Bean 注册方式
            1. 结构体指针
            2. 构造函数
                1. func(…)Bean
                2. func(…)(Bean,error)
                3. 返回值可以是值类型或者引用类型
        3. 构造函数参数模型
            1. 可注入:
                1. Bean
                2. 配置项
            2. 固定值
            3. option 函数
                1. option 参数同样支持注入/固定值/条件
            4. 参数绑定方式
                1. 按顺序
                2. 指定下标（可省略自动注入参数）
            5. 为参数指定条件
                1. 控制参数是否参与注入
        4. 生命周期
            1. init/destroy
                1. 函数或方法名
                2. func(bean)
                3. func(bean)error
            2. 失败语义
                1. init 失败 → 容器启动失败，进程退出
                    1. 已创建资源不 destroy
                2. destroy 失败 → 仅记录日志，不阻塞后续流程
            3. 生命周期状态机
                1. 默认 -> ...
        5. 依赖与导出
            1. 显式接口导出
                1. 不自动按 assignable 推导
            2. 依赖其他 Bean
                1. 通过 Bean ID 指定
            3. 原始类型 Bean 与接口 Bean 视为不同实体
        6. 条件与可见性
            1. Bean 成立条件
                1. OnProfiles
                2. 通用 Condition
            2. Bean 可见性
                1. 搭配 go internal 控制
        7. Configuration / 子Bean
            1. 一个 Bean 通过函数导出子 Bean
            2. include / exclude 控制导出范围
    2. Module
        1. func(Properties,BeanProvider)error
        2. 条件化控制一组 Bean 的注册
        3. Module 中的 Bean 直接注册到容器
        4. Starter 机制的完美抽象
    3. Condition
        1. 内建条件
            1. 属性是否存在
            2. Bean 是否存在
        2. 组合条件
            1. not / and / or / none
        3. 自定义条件
3. 注册模型
    1. 全局注册阶段
        1. 只能在 init 函数中调用
        2. 全局 Bean / Module 注册
        3. 单测场景下的容器隔离基础
    2. 容器注册阶段
        1. 容器级 Bean 注册
        2. Module 中 Bean 注册
        3. 单测场景下注入自定义 Bean
4. 解析模型
    1. Bean 合并
        1. 全局 Bean
        2. 容器 Bean
        3. Module 中注册的容器 Bean
        4. Configuration 导出的子 Bean
    2. Bean 解析
        1. 条件裁剪
            1. 条件不满足的 Bean 被删除
        2. 冲突检测
            1. 类型 + 名称完全一致视为冲突
        3. 接口 Bean 与原始 Bean 视为不同的 Bean
5. 注入与构建模型
    1. 按需创建
        1. 从 root beans 开始遍历
            1. root bean 是一种特殊的 bean，可以类比为树的根
            2. app server 中的 runners 和 servers 是一种 root bean
            3. RunAsync 和 RunTest 入参也注册为 root bean
        2. 未被依赖的 Bean 不创建
    2. 注入方式
        1. 构造函数注入
        2. 结构体字段注入
    3. 注入目标
        1. 数量
            1. 单 Bean
            2. 多 Bean
                1. slice（支持顺序）
                2. map
        2. 类型
            1. 原始类型
            2. 接口类型
        3. 可空注入
            1. Testing 模式下默认可空
        4. Tag 语法
            1. autowire、inject (二者等价)
                1. a? 单 bean 注入，可空
                2. a?,b?,*? 多 bean 注入，可空
                3. ${key} 支持通过配置指定注入目标
            2. value
                1. 配置项绑定
            3. value 与 autowire 同阶段完成
    4. Bean 创建
        1. 构造函数返回 error → 启动失败
        2. 已创建 Bean 不 destroy
    5. 循环依赖
        1. 支持字段注入循环依赖
            1. 通过 lazy 打破
        2. 不支持构造函数循环依赖
    6. Lazy 注入
        1. 非 lazy Bean 完成后统一注入
    7. Destroy 顺序
        1. 被依赖者先 init，后 destroy
    8. Bean 冲突策略
        1. 不允许 Bean 覆盖
        2. 不支持 primary
        3. 通过条件实现互斥 Bean
    9. 启动期模型
        1. 仅启动期注入
        2. 不提供运行时获取 Bean 的 API
        3. 注入完成后清理中间数据
    10. Context 注入
        1. 通过 ContextAware 获取根 Context
    11. 并发模型
        1. 单线程、顺序注入
    12. 最佳实践
        1. 注入时尽量不指定 Bean 名称，通过类型进行注入
6. 可视化与诊断
    1. 启动期记录：
        1. Bean 解析过程
        2. 条件裁剪结果
        3. 注入路径
    2. 用于调试与问题定位

# 四、启停

1. 设计原则与约束
    1. 启动流程是线性的、不可回滚的
    2. init 阶段仅允许注册元数据，不允许执行逻辑
    3. 启动失败即进程退出，不清理已初始化资源
    4. 仅支持启动期依赖注入，不支持运行时注入
    5. 长期运行行为必须封装为 server
    6. runner 为一次性执行单元，不允许启动后台 goroutine
    7. 所有生命周期必须受 root ctx 约束
    8. 日志必须在进程退出前完成 flush
    9. server ready 之后发生异常进入 shutdown 流程
2. init 阶段（元数据注册）
    1. 注册全局 bean 和 module
    2. 不建议做任何其他事情
        1. init 的唯一价值是注册各种元数据
        2. 不执行具体逻辑、不触发副作用
3. 启动主流程（顺序执行）
    1. App 构建与配置阶段
        1. 创建 app 对象
        2. 打印 banner
            1. 支持自定义 banner 内容
        3. 执行 configure 回调
            1. 设置容器级 property
            2. 设置容器级 bean
            3. 目的是支持特殊场景下自定义容器内容
    2. 配置初始化阶段
        1. 合并完整配置
            1. 命令行参数
            2. 环境变量
            3. 含 import 的配置文件
            4. 代码配置
        2. 配置失败
            1. 立即终止启动
            2. 不清理已初始化资源
    3. 日志初始化阶段
        1. 解析 logging 配置
        2. 初始化全局日志组件
    4. IoC 容器初始化阶段
        1. 注册 app 为 root bean
        2. 注册 ContextAware
            1. 用于获取 app 的 root ctx
            2. 禁止业务代码中出现 background 或者 todo ctx
        3. 注册 ConfigRefresher
            1. 用于实现动态配置刷新
        4. 刷新 IoC 容器
            1. 包含 properties 与 root beans
            2. 从 root beans 开始按需注入
        5. 通过注入方式收集 runner 和 server
        6. IoC 刷新成功后释放非必要资源
            1. 仅保留动态配置、destroy 等必需资源
    5. 启动执行阶段
        1. Runner 执行
            1. 顺序执行所有 runner
                1. 默认收集顺序不固定
                2. 可通过配置项指定顺序
            2. runner 执行完即结束
            3. runner 执行失败
                1. 程序直接退出
                2. 不清理已初始化资源
            4. 原则上 runner 间不应存在依赖关系
            5. runner 中不应启动后台 goroutine
                1. 否则应封装为 server
        2. Server 启动
            1. 并行启动所有 server
            2. start 过程中 panic 视为 error
            3. 需要 stop 能力的组件必须封装为 server
            4. 支持统一 ready 信号
                1. socket 优先完成 listen
                2. listen 失败 → 程序退出
            5. ready 信号之后
                1. socket 开始 read
                2. 请求处理正式开始
            6. 所有 server ready
                1. 视为启动成功
4. 运行期与退出流程
    1. Exit 监听与 Shutdown
        1. 监听 exit 信号
            1. 一般为 Ctrl+C
        2. 调用 app shutdown
            1. 幂等
            2. 可重复调用
            3. 可监听 Exiting 标记
    2. Server 停止
        1. 并行执行 server stop
        2. 等待所有 server 成功退出
        3. 不支持 timeout
            1. 避免请求处理中断
    3. 资源释放与进程退出
        1. 关闭 IoC 容器
            1. destroy 容器持有的资源
        2. 停止日志组件（defer）
            1. 等待日志 flush
            2. 释放日志资源
        3. 确保日志 flush 完成后进程退出
5. 启动模式
    1. Run
        1. 一个函数完成完整启停流程
        2. 自动监听 exit 信号
    2. RunAsync
        1. 适用于旧项目改造
        2. 需手动调用 stop 函数
6. Context 注入
    1. 提供 root ctx
    2. 用于禁止 Background ctx 的使用
7. 生命周期回调
    1. 当前不支持，未来如有必要再引入
8. 调试与日志
    1. 支持启停关键节点日志输出

# 五、内置 HTTP Server

1. 定位与设计原则
    1. 兼容 go 标准库内置的全局 http server
    2. 不提供高阶 Web 能力
        1. 不提供 Web 框架级上下文对象
        2. 不提供参数绑定、返回值自动序列化
        3. 不提供路由分组、路由优先级控制
        4. 不提供模板渲染能力
    3. 生命周期纳入 go-spring 启停模型
        1. start / stop 统一管理
        2. 支持优雅 shutdown
2. http server 实现
    1. 支持 http 默认的全局路由处理器
        1. 兼容 go 标准库最佳实践
    2. 支持自定义 http 路由处理器
        1. 基于 http.Handler / HandlerFunc
    3. 支持标准的 http 中间件模式
        1. func(http.Handler) http.Handler
    4. 支持通过配置禁用内置 http server
    5. 支持自定义 http server 配置项
        1. 端口号、各种超时

# 六、组件

1. starter 机制
    1. 注册方式
        1. 基于 `init` 函数注册 bean 或 module
            1. 支持空白导入
            2. 甚至无需特殊导入，包能看到即可注册
    2. 注册形式
        1. provide 形式
            1. 一个 starter 可以注册多个 bean
            2. 使用 `Provide` 函数注册 bean
        2. module 形式
            1. 一个 starter 可以注册多个 module
            2. 一个 module 可以注册多个 bean
            3. module 通过 `Provide` 函数注册 bean
        3. group 形式
            1. module 的特殊形式
            2. 适用于资源型 bean，例如注册多个 redis 实例
    3. 按需实例化
        1. IoC 容器支持按需实例化和注入
        2. 即使引入组件，在未被使用的情况下也不会创建实例
    4. 配置驱动
        1. 推荐通过配置（或环境变量）启用 / 禁用组件
        2. 不推荐基于 bean 依赖是否存在来启用 / 禁用组件
    5. 自定义 starter 组件
2. 资源型组件
    1. MySQL 组件
        1. gorm
    2. Redis 组件
        1. redigo
        2. go-redis
3. server 型组件
    1. HTTP server
    2. gRPC server
    3. Thrift server
4. pprof 组件
    1. pprof server
    2. 持续性能分析

# 七、测试

1. 测试基础
    1. 使用 `go test` 原生测试机制
2. 普通测试
    1. 不使用 IoC 容器
    2. 使用常规单元测试技术
    3. 支持对接口、方法、函数进行 mock
    4. 推荐使用依赖注入技术
3. IoC 测试
    1. 使用 IoC 容器
    2. 通过极简的 `RunTest(...)` 启动测试
    3. 数据隔离
        1. 拷贝 `init` 阶段注册的 bean
        2. 支持注册容器级别的 bean
    4. 支持对接口、方法、函数进行 mock
4. 并行测试
    1. 普通测试支持并行执行
    2. IoC 测试仅支持依次执行
        1. 因为很可能共享全局状态
5. 断言
    1. 流式断言风格
    2. 类型断言
        1. panic
        2. that
        3. error
        4. string
        5. number
        6. slice
        7. map
    3. 两种断言模式
        1. `assert`：断言失败不立即退出
        2. `require`：断言失败立即退出
6. mock
    1. 接口 mock
        1. 基于代码生成技术
    2. 方法 / 函数 mock
        1. 基于 monkey 技术
    3. 泛型支持
        1. 泛型接口
        2. 泛型函数
    4. IDE 类型提示
        1. 基于泛型类型
    5. 支持可变参数
    6. 并行安全
        1. 基于 `ctx` 实现数据隔离
    7. 行为替换
        1. 行为验证由用户自行完成

# 八、http-gen

1. 项目结构与依赖
    1. meta.json
    2. IDL 文件
    3. import 机制
        1. 引入外部 IDL 文件
    4. 共享命名空间
2. IDL 语法与语义
    1. 注释
        1. 单行注释
        2. 多行注释
    2. 关键字
        1. extends
        2. const
        3. enum
        4. type
        5. oneof
        6. rpc
        7. sse
        8. true/false
        9. optional
        10. required
    3. 基础类型
        1. bool
        2. int
            1. 不内置 uint 类型
            2. 可自定义 uint 类型
        3. float
        4. string
        5. bytes
        6. list、map
            1. 支持多层嵌套
    4. 常量
        1. const TYPE NAME = VALUE
    5. 注解
        1. 单行注解
        2. 多行注解
        3. 基本元素
            1. key(=value)?
                1. value 缺省时表示 bool 语义
    6. 枚举
        1. 普通枚举
        2. 错误码枚举
            1. errmsg 注解
            2. extends 扩展
                1. 错误码合并
        3. enum_as_string
    7. 类型（结构体）
        1. 结构体类型
            1. 普通结构体
            2. 泛型结构体
            3. 泛型实例化
        2. 字段类型
            1. 普通字段
            2. 泛型字段
            3. 嵌入字段
        3. 字段限定符
            1. 可选 (optional)
            2. 必选 (required)
        4. 字段注解
            1. 自定义类型
            2. 字段校验
            3. 参数绑定
            4. 序列化
            5. 其他
        5. 嵌入类型
            1. 字段合并
        6. 联合类型
            1. oneof = enum + struct
        7. 字段校验
            1. 内置函数
            2. 自定义函数
            3. 运算符和优先级
            4. 特殊变量
                1. $ 当前字段值
3. 接口定义
    1. 接口类型
        1. rpc 接口
        2. sse 接口（流式响应）
    2. 接口注解
        1. method、path
        2. contentType
        3. form
        4. json
        5. timeouts
        6. 自定义响应类型
    3. Restful Path
        1. 路径风格
            1. :name / :name…
            2. {name} / {name…}
        2. 参数绑定
            1. 仅支持 int、string
            2. 字段要求必须
4. 代码生成器
    1. Go 代码生成器
    2. 流式字段解析
        1. 支持 required 校验
        2. 流式 json 解析
        3. 流式 form 解析


