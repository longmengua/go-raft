package convert

import "fmt"

func ToInt(val any) (int, error) {
	switch v := val.(type) {
	case int:
		return v, nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float32:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		var n int
		_, err := fmt.Sscanf(v, "%d", &n)
		if err != nil {
			return 0, fmt.Errorf("cannot parse string to int: %v", err)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("unsupported type %T", val)
	}
}
