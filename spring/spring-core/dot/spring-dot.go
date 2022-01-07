package dot

import (
	"bytes"
	"fmt"
	SpringCore "github.com/go-spring/spring-core"
	"io"
	"reflect"
	"strings"
)

type RootGraph struct {
	//edges 边集合，边用于表示依赖关系
	edges []Edge
	//graphs 子图集合，每一个 bean 作为一个 子图
	graphs []SubGraph
}

//SubGraph 每个子图表示一个 bean
type SubGraph struct {
	//子图名，子图必须以 cluster 开头，这里不存 cluster
	name string
	//nodes 节点用于表示 struct bean 的属性字段
	nodes []Node
	//用于布局的边
	edges []Edge
}

type Shape string

type Node struct {
	//name 节点名，声明时使用，取 bean的name
	name string
	//shape 节点形状, 现在用于区分各种数据类型, 参考 typeShapeMapping 声明
	shape Shape
	//label 做展示量，不存在时取 name 作为展示值
	//1.用于简化非 bean 字段
	//2.用于数组展示
	//3.用于 map 展示
	label string
}

type Edge struct {
	//fromName 从此节点出发
	fromName string
	//toName 节点指向 node
	toName string
	hidden bool
}

//GraphContext 图形上下文，生成图形时使用
type GraphContext struct {
	beans       []*SpringCore.BeanDefinition
	graphs      []SubGraph
	edges       []Edge
	typeNameMap map[reflect.Type]string
}

/*
build 构建完整图
GraphContext 生成时会新建一个 SubGraph 列表和 Edge 列表在输出时使用
另外会新建一个以 beanTypeName 为 key，以 beanName 为 value 的 map 在新建 Edge 时使用
build 时会将依据 bean 生成 SubGraph，而后根据 beanRealType 去生成不同的图结构
定义 beanRealType 在 beanType 为 reflect.Ptr 时为指向类型，否则为 beanType

每个 bean 都生成一个 cluster_beanName 子图，并生成一个 beanName 节点

当 beanRealType 为 struct 时, beanName 节点 Node.label 为 beanType
而后依其全部 field 生成节点
若 field 声明 autowire 则创建 Edge 表示依赖
    若 autowire tag value 不为空取其值为 Edge.toName
	否则取 field type 在 GraphContext.typeNameMap 存储的名字为 Edge.toName
	设置 nodeName 为 fieldName
否则设置 nodeName 为 beanName_fieldName, label 为 fieldName:fieldType
创建field num 个 Edge 用于布局 其 Edge.hidden 为 true，其 Edge.fromName 为 beanName, 其 Edge.toName 为每个node name

当 beanRealType 为 array or slice 时， node type 为 NodeShapeRecord, label 将展示元素类型与其 length

当 beanRealType 为 map 时， node type 为 NodeShapeMrecord, label 将展示 map 声明 与其 length
 */
func (gc *GraphContext) build() {
	//构建 bean SubGraph 并记录 type - name
	for i := range gc.beans {
		beanType := reflect.TypeOf(gc.beans[i].Bean())
		beanRealType := realType(beanType)
		gc.graphs[i] = SubGraph{
			name: gc.beans[i].Name(),
		}
		if beanRealType.Kind() == reflect.Struct {
			gc.buildCommonNode(i, beanRealType)
		} else if beanRealType.Kind() == reflect.Array ||
			beanRealType.Kind() == reflect.Slice {
			gc.buildArrayNode(i, beanRealType)
		} else if beanRealType.Kind() == reflect.Map {
			gc.buildMapNode(i)
		}
	}
}

func (gc *GraphContext) buildCommonNode(i int, beanRealType reflect.Type) {
	nodes := make([]Node, beanRealType.NumField()+1)
	gc.graphs[i].edges = make([]Edge, beanRealType.NumField())
	nodes[0] = Node{
		name:  gc.beans[i].Name(),
		shape: NodeShapePlaintext,
		label: gc.beans[i].TypeName(),
	}
	for j := 0; j < beanRealType.NumField(); j++ {
		field := beanRealType.Field(j)
		fieldRealType := realType(field.Type)
		nodes[j+1] = Node{shape: getShape(fieldRealType.Kind())}
		tagVal, hasTag := field.Tag.Lookup("autowire")
		if hasTag {
			var toName string
			// 构建 edge
			if tagVal == "" {
				toName = gc.typeNameMap[field.Type]
			} else {
				toName = tagVal
			}
			nodes[j + 1].name = field.Name
			gc.edges = append(gc.edges, Edge{fromName: field.Name, toName: toName})
		} else {
			nodes[j+1].label = field.Name + ":" + field.Type.Name()
			nodes[j+1].name = gc.beans[i].Name() + "_" + field.Name
		}
		gc.graphs[i].edges[j] = Edge{
			fromName: nodes[0].name,
			toName:   nodes[j+1].name,
			hidden:   true,
		}
	}
	gc.graphs[i].nodes = nodes
}

func (gc *GraphContext) buildArrayNode(i int, beanRealType reflect.Type) {
	label := fmt.Sprintf("type\\n%s|length\\n%d", beanRealType.Elem(), gc.beans[i].Value().Len())
	label = strings.ReplaceAll(label, "{", "\\{")
	label = strings.ReplaceAll(label, "}", "\\}")
	gc.graphs[i].nodes = []Node{{
		name:  gc.beans[i].Name(),
		shape: NodeShapeRecord,
		label: label,
	}}
}

func (gc *GraphContext) buildMapNode(i int) {
	label := fmt.Sprintf("%s\\n|length:%d", gc.beans[i].TypeName(), gc.beans[i].Value().Len())
	label = strings.ReplaceAll(label, "{", "\\{")
	label = strings.ReplaceAll(label, "}", "\\}")
	gc.graphs[i].nodes = []Node{
		{
			name:  gc.beans[i].Name(),
			shape: NodeShapeMrecord,
			label: label,
		}}
}

func newGraphContext(beans []*SpringCore.BeanDefinition) *GraphContext {
	graphContext := &GraphContext{
		beans:       beans,
		graphs:      make([]SubGraph, len(beans)),
		edges:       make([]Edge, 0, len(beans)),
		typeNameMap: make(map[reflect.Type]string),
	}
	for i := range beans {
		graphContext.typeNameMap[reflect.TypeOf(beans[i].Bean())] = beans[i].Name()
	}
	return graphContext
}

func WithContext(ctx SpringCore.SpringContext) *RootGraph {
	graphContext := newGraphContext(ctx.GetBeanDefinitions())
	graphContext.build()
	return &RootGraph{
		edges:  graphContext.edges,
		graphs: graphContext.graphs,
	}
}

func realType(aType reflect.Type) reflect.Type {
	if aType.Kind() == reflect.Ptr {
		return aType.Elem()
	}
	return aType
}

func getShape(kind reflect.Kind) Shape {
	if shape, have := typeShapeMapping[kind]; have {
		return shape
	}
	return NodeShapeEllipse
}

func (root *RootGraph) WriteDot(writer io.Writer) error {
	buffer := &bytes.Buffer{}
	buffer.Write([]byte(fmt.Sprintf("digraph {compound=true;rankdir=TB;\n")))
	for _, graph := range root.graphs {
		graph.writeDot(buffer, 2)
	}
	for _, edge := range root.edges {
		edge.writeDot(buffer, 2)
	}
	buffer.Write([]byte("}\n"))
	_, err := writer.Write(buffer.Bytes())
	return err
}

func (graph *SubGraph) writeDot(buffer *bytes.Buffer, offset int) {
	writeWithOffset(buffer, offset, fmt.Sprintf("subgraph \"cluster_%s\" {\n", graph.name))
	writeWithOffset(buffer, offset+2, fmt.Sprintf("graph[label=\"%s\"]\n", graph.name))
	for _, node := range graph.nodes {
		node.writeDot(buffer, offset+2)
	}
	for _, edge := range graph.edges {
		edge.writeDot(buffer, offset+2)
	}
	writeWithOffset(buffer, offset, "}\n")
}

func (node *Node) writeDot(buffer *bytes.Buffer, offset int) {
	buffer.Write(make([]byte, offset))
	if node.label == "" {
		writeWithOffset(buffer, offset, fmt.Sprintf("\"%s\"[shape = %s]\n", node.name, node.shape))
	} else {
		writeWithOffset(buffer, offset, fmt.Sprintf("\"%s\"[shape = %s,label = \"%s\"]\n", node.name, node.shape, node.label))
	}
}

func (edge *Edge) writeDot(buffer *bytes.Buffer, offset int) {
	if edge.hidden {
		writeWithOffset(buffer, offset, fmt.Sprintf("\"%s\" -> \"%s\"[lhead=\"cluster_%s\",style=invis]\n",
			edge.fromName, edge.toName, edge.toName))
	} else {
		writeWithOffset(buffer, offset, fmt.Sprintf("\"%s\" -> \"%s\"[lhead=\"cluster_%s\"]\n",
			edge.fromName, edge.toName, edge.toName))
	}
}

func writeWithOffset(buffer *bytes.Buffer, offset int, val string) {
	b := make([]byte, offset)
	for i := 0; i < offset; i++ {
		b[i] = ' '
	}
	buffer.Write(b)
	buffer.Write([]byte(val))
}