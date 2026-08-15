package tools

import (
	"errors"
	"fmt"

	"github.com/Hana-ame/api-pack/tools/orderedmap"
)

// 定义哨兵错误
var ErrKeyNotExists = errors.New("键不存在")

func Extract[T any](o *orderedmap.OrderedMap, keys ...string) (v T, err error) {
	if len(keys) == 0 {
		return v, fmt.Errorf("至少需要提供一个 key")
	}

	current := o // 当前查找的节点

	// 遍历除最后一个 key 外的所有中间 key
	for i := 0; i < len(keys)-1; i++ {
		key := keys[i]
		value, exists := current.Get(key)
		if !exists {
			return v, fmt.Errorf("键路径中断: 缺少 '%s'", key)
		}

		// 检查中间节点类型
		nextMap, ok := value.(orderedmap.OrderedMap)
		if !ok {
			return v, fmt.Errorf("类型错误: '%s' 不是嵌套对象", key)
		}
		current = &nextMap
	}

	// 处理最后一个 key
	lastKey := keys[len(keys)-1]
	value, exists := current.Get(lastKey)
	if !exists {
		// 生成错误时用 %w 包装
		err = fmt.Errorf("%w: '%s'", ErrKeyNotExists, lastKey)
		return v, err
	}

	// 类型断言
	target, ok := value.(T)
	if !ok {
		return v, fmt.Errorf("类型不匹配: 期望 %T, 实际 %T", *new(T), value)
	}

	return target, nil
}

func ExtractInSequence[T any](o *orderedmap.OrderedMap, keysSequence ...[]string) (v T, err error) {
	for _, keys := range keysSequence {
		v, err = Extract[T](o, keys...)
		if err == nil {
			return
		}
	}
	return
}
