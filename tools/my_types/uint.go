package mytypes

type Uint interface {
	Value() uint64

	ICompareable[Uint]
	ICompareableAlt[Uint]
}

type Uint64 uint64

func (i Uint64) Value() uint64 {
	return uint64(i)
}

func (i Uint64) CompareTo(other Uint) int {
	if uint64(i) < uint64(other.Value()) {
		return -1
	} else if uint64(i) > uint64(other.Value()) {
		return 1
	} else {
		return 0
	}
}

func (i Uint64) EQ(that Uint) bool {
	return uint64(i) == uint64(that.Value())
}
func (i Uint64) GT(that Uint) bool {
	return uint64(i) > uint64(that.Value())
}
func (i Uint64) LT(that Uint) bool {
	return uint64(i) < uint64(that.Value())
}
func (i Uint64) GTE(that Uint) bool {
	return uint64(i) >= uint64(that.Value())
}
func (i Uint64) LTE(that Uint) bool {
	return uint64(i) <= uint64(that.Value())
}

type Uint32 int32

func (i Uint32) Value() uint64 {
	return uint64(i)
}

func (i Uint32) CompareTo(other Uint) int {
	if uint64(i) < uint64(other.Value()) {
		return -1
	} else if uint64(i) > uint64(other.Value()) {
		return 1
	} else {
		return 0
	}
}

func (i Uint32) EQ(that Uint) bool {
	return uint64(i) == uint64(that.Value())
}
func (i Uint32) GT(that Uint) bool {
	return uint64(i) > uint64(that.Value())
}
func (i Uint32) LT(that Uint) bool {
	return uint64(i) < uint64(that.Value())
}
func (i Uint32) GTE(that Uint) bool {
	return uint64(i) >= uint64(that.Value())
}
func (i Uint32) LTE(that Uint) bool {
	return uint64(i) <= uint64(that.Value())
}

type Uint16 int16

func (i Uint16) Value() uint64 {
	return uint64(i)
}

func (i Uint16) CompareTo(other Uint) int {
	if uint64(i) < uint64(other.Value()) {
		return -1
	} else if uint64(i) > uint64(other.Value()) {
		return 1
	} else {
		return 0
	}
}

func (i Uint16) EQ(that Uint) bool {
	return uint64(i) == uint64(that.Value())
}
func (i Uint16) GT(that Uint) bool {
	return uint64(i) > uint64(that.Value())
}
func (i Uint16) LT(that Uint) bool {
	return uint64(i) < uint64(that.Value())
}
func (i Uint16) GTE(that Uint) bool {
	return uint64(i) >= uint64(that.Value())
}
func (i Uint16) LTE(that Uint) bool {
	return uint64(i) <= uint64(that.Value())
}

type Uint8 int8

func (i Uint8) Value() uint64 {
	return uint64(i)
}

func (i Uint8) CompareTo(other Uint) int {
	if uint64(i) < uint64(other.Value()) {
		return -1
	} else if uint64(i) > uint64(other.Value()) {
		return 1
	} else {
		return 0
	}
}

func (i Uint8) EQ(that Uint) bool {
	return uint64(i) == uint64(that.Value())
}
func (i Uint8) GT(that Uint) bool {
	return uint64(i) > uint64(that.Value())
}
func (i Uint8) LT(that Uint) bool {
	return uint64(i) < uint64(that.Value())
}
func (i Uint8) GTE(that Uint) bool {
	return uint64(i) >= uint64(that.Value())
}
func (i Uint8) LTE(that Uint) bool {
	return uint64(i) <= uint64(that.Value())
}
