package mytypes

type String string

func (s String) Value() string {
	return string(s)
}
func (s String) String() string {
	return string(s)
}

func (i String) CompareTo(other String) int {
	if string(i) < string(other.Value()) {
		return -1
	} else if string(i) > string(other.Value()) {
		return 1
	} else {
		return 0
	}
}
func (s String) EQ(that String) bool {
	return string(s) == string(that.Value())
}
func (s String) GT(that String) bool {
	return string(s) > string(that.Value())
}
func (s String) LT(that String) bool {
	return string(s) < string(that.Value())
}
func (s String) GTE(that String) bool {
	return string(s) >= string(that.Value())
}
func (s String) LTE(that String) bool {
	return string(s) <= string(that.Value())
}
