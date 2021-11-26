package cast

type TimeCastOption struct {
	Format string
}

func (o *TimeCastOption) GetFormat() string {
	if o == nil {
		return ""
	}
	return o.Format
}

func getTimeCastOption(options ...*TimeCastOption) *TimeCastOption {
	if len(options) <= 0 {
		return nil
	}
	return options[0]
}
