package sort

import (
	"container/list"
	"errors"
)

// TripleSorting 三路排序
func TripleSorting(sorting *list.List, fn GetBeforeItems) *list.List {

	// 待排列表
	toSort := list.New()
	toSort.PushBackList(sorting)

	// 已排序列表
	sorted := list.New()

	// 正在处理的列表
	processing := list.New()

	for toSort.Len() > 0 { // 每次循环选出依赖链条最前端的元素
		tripleSortByAfter(sorting, toSort, sorted, processing, nil, fn)
	}
	return sorted
}

// tripleSortByAfter 递归选出依赖链条最前端的元素
func tripleSortByAfter(sorting *list.List, toSort *list.List, sorted *list.List,
	processing *list.List, current interface{}, fn GetBeforeItems) {

	// 选出待排元素
	if current == nil {
		current = toSort.Remove(toSort.Front())
	}

	processing.PushBack(current)

	// 选出排在当前项前面的元素
	for e := fn(sorting, current).Front(); e != nil; e = e.Next() {
		c := e.Value

		// 自己不可能是自己前面的元素，除非出现了循环依赖，因此抛出 Panic
		for p := processing.Front(); p != nil; p = p.Next() {
			if p.Value == c {
				panic(errors.New("found sorting cycle"))
			}
		}

		inSorted := false
		for p := sorted.Front(); p != nil; p = p.Next() {
			if p.Value == c {
				inSorted = true
				break
			}
		}

		inToSort := false
		for p := toSort.Front(); p != nil; p = p.Next() {
			if p.Value == c {
				inToSort = true
				break
			}
		}

		if !inSorted && inToSort { // 递归处理当前 Destroyer 的依赖并进行排序
			tripleSortByAfter(sorting, toSort, sorted, processing, c, fn)
		}
	}

	// 排序完成，从正在排序、待排序列表删除，然后添加到已排序列表
	{
		for p := processing.Front(); p != nil; p = p.Next() {
			if p.Value == current {
				processing.Remove(p)
				break
			}
		}

		for p := toSort.Front(); p != nil; p = p.Next() {
			if p.Value == current {
				toSort.Remove(p)
				break
			}
		}

		sorted.PushBack(current)
	}
}

// GetBeforeItems 获取 sorting 中排在 current 前面的项
type GetBeforeItems func(sorting *list.List, current interface{}) *list.List
