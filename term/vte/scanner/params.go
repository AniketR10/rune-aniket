package scanner

type params struct {
	subparams        [MaxParams]uint16
	params           [MaxParams]uint16
	currentSubparams uint16
	len              uint16
}

func (p *params) isFull() bool {
	return p.len == MaxParams
}

func (p *params) push(item uint16) {
	p.subparams[p.len-p.currentSubparams] = p.currentSubparams + 1
	p.params[p.len] = item
	p.currentSubparams = 0
	p.len += 1
}

func (p *params) extend(item uint16) {
	p.subparams[p.len-p.currentSubparams] = p.currentSubparams + 1
	p.params[p.len] = item
	p.currentSubparams += 1
	p.len += 1
}

func (p *params) slice() (ret [][]uint16) {
	index := uint16(0)
	for index < p.len {
		numSubparams := p.subparams[index]
		param := p.params[index : index+numSubparams]
		ret = append(ret, param)
		index += numSubparams
	}
	return
}

func (p *params) reset() {
	p.currentSubparams = 0
	p.len = 0
}
