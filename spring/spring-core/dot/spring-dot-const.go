package dot

import "reflect"

//使用 NodeShapeEgg (egg) 表示 interface
//使用 NodeShapeBox (record) 表示 struct
//使用 NodeShapeRecord (record) 表示 array, slice
//使用 NodeShapeMrecord (Mrecord) 表示 map
//其余使用默认值 NodeShapeEllipse (ellipse) 椭圆
const (
	//NodeShapeMrecord 圆角组合矩形
	NodeShapeMrecord = Shape("Mrecord")
	//NodeShapeRecord 组合矩形
	NodeShapeRecord = Shape("record")
	//NodeShapeBox 普通矩形
	NodeShapeBox = Shape("box")
	//NodeShapeEgg 蛋形
	NodeShapeEgg = Shape("egg")
	//NodeShapePlaintext 无形状边框，纯文本
	NodeShapePlaintext = Shape("plaintext")
	//NodeShapeEllipse 椭圆, 默认椭圆
	NodeShapeEllipse = Shape("ellipse")
)

var typeShapeMapping = map[reflect.Kind]Shape{
	reflect.Array: NodeShapeRecord,
	reflect.Map: NodeShapeMrecord,
	reflect.Slice: NodeShapeRecord,

	reflect.Struct: NodeShapeBox,

	reflect.Interface: NodeShapeEgg,
}
