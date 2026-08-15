package mytypes

type Float interface {
	Value() float64

	ICompareable[Float]
	ICompareableAlt[Float]
}

type Float64 float64

func (i Float64) Value() float64 {
	return float64(i)
}

func (i Float64) CompareTo(other Float) int {
	if float64(i) < float64(other.Value()) {
		return -1
	} else if float64(i) > float64(other.Value()) {
		return 1
	} else {
		return 0
	}
}

func (i Float64) EQ(that Float) bool {
	return float64(i) == float64(that.Value())
}
func (i Float64) GT(that Float) bool {
	return float64(i) > float64(that.Value())
}
func (i Float64) LT(that Float) bool {
	return float64(i) < float64(that.Value())
}
func (i Float64) GTE(that Float) bool {
	return float64(i) >= float64(that.Value())
}
func (i Float64) LTE(that Float) bool {
	return float64(i) <= float64(that.Value())
}

type Float32 float32

func (i Float32) Value() float64 {
	return float64(i)
}

func (i Float32) CompareTo(other Float) int {
	if float64(i) < float64(other.Value()) {
		return -1
	} else if float64(i) > float64(other.Value()) {
		return 1
	} else {
		return 0
	}
}

func (i Float32) EQ(that Float) bool {
	return float64(i) == float64(that.Value())
}
func (i Float32) GT(that Float) bool {
	return float64(i) > float64(that.Value())
}
func (i Float32) LT(that Float) bool {
	return float64(i) < float64(that.Value())
}
func (i Float32) GTE(that Float) bool {
	return float64(i) >= float64(that.Value())
}
func (i Float32) LTE(that Float) bool {
	return float64(i) <= float64(that.Value())
}
