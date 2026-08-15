package mytypes

import "strconv"

type Int interface {
	Value() int64

	ICompareable[Int]
	ICompareableAlt[Int]
}

type Int64 int64

func (i Int64) Value() int64 {
	return int64(i)
}
func (i Int64) String() string {
	return strconv.FormatInt(int64(i), 10)
}

func (i Int64) CompareTo(other Int) int {
	if int64(i) < int64(other.Value()) {
		return -1
	} else if int64(i) > int64(other.Value()) {
		return 1
	} else {
		return 0
	}
}

func (i Int64) EQ(that Int) bool {
	return int64(i) == int64(that.Value())
}
func (i Int64) GT(that Int) bool {
	return int64(i) > int64(that.Value())
}
func (i Int64) LT(that Int) bool {
	return int64(i) < int64(that.Value())
}
func (i Int64) GTE(that Int) bool {
	return int64(i) >= int64(that.Value())
}
func (i Int64) LTE(that Int) bool {
	return int64(i) <= int64(that.Value())
}

type Int32 int32

func (i Int32) Value() int64 {
	return int64(i)
}
func (i Int32) String() string {
	return strconv.FormatInt(int64(i), 10)
}

func (i Int32) CompareTo(other Int) int {
	if int64(i) < int64(other.Value()) {
		return -1
	} else if int64(i) > int64(other.Value()) {
		return 1
	} else {
		return 0
	}
}

func (i Int32) EQ(that Int) bool {
	return int64(i) == int64(that.Value())
}
func (i Int32) GT(that Int) bool {
	return int64(i) > int64(that.Value())
}
func (i Int32) LT(that Int) bool {
	return int64(i) < int64(that.Value())
}
func (i Int32) GTE(that Int) bool {
	return int64(i) >= int64(that.Value())
}
func (i Int32) LTE(that Int) bool {
	return int64(i) <= int64(that.Value())
}

type Int16 int16

func (i Int16) Value() int64 {
	return int64(i)
}
func (i Int16) String() string {
	return strconv.FormatInt(int64(i), 10)
}

func (i Int16) CompareTo(other Int) int {
	if int64(i) < int64(other.Value()) {
		return -1
	} else if int64(i) > int64(other.Value()) {
		return 1
	} else {
		return 0
	}
}

func (i Int16) EQ(that Int) bool {
	return int64(i) == int64(that.Value())
}
func (i Int16) GT(that Int) bool {
	return int64(i) > int64(that.Value())
}
func (i Int16) LT(that Int) bool {
	return int64(i) < int64(that.Value())
}
func (i Int16) GTE(that Int) bool {
	return int64(i) >= int64(that.Value())
}
func (i Int16) LTE(that Int) bool {
	return int64(i) <= int64(that.Value())
}

type Int8 int8

func (i Int8) Value() int64 {
	return int64(i)
}
func (i Int8) String() string {
	return strconv.FormatInt(int64(i), 10)
}

func (i Int8) CompareTo(other Int) int {
	if int64(i) < int64(other.Value()) {
		return -1
	} else if int64(i) > int64(other.Value()) {
		return 1
	} else {
		return 0
	}
}

func (i Int8) EQ(that Int) bool {
	return int64(i) == int64(that.Value())
}
func (i Int8) GT(that Int) bool {
	return int64(i) > int64(that.Value())
}
func (i Int8) LT(that Int) bool {
	return int64(i) < int64(that.Value())
}
func (i Int8) GTE(that Int) bool {
	return int64(i) >= int64(that.Value())
}
func (i Int8) LTE(that Int) bool {
	return int64(i) <= int64(that.Value())
}
