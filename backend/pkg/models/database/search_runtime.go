package database

import (
	"fmt"
	"sync"

	"github.com/dop251/goja"
)

var searchPrograms struct {
	sync.Once
	fhir, extractor *goja.Program
	err             error
}

// Keep only a small number of idle runtimes. A checked-out runtime belongs to
// exactly one extraction; Goja runtimes must never run concurrently.
var idleSearchRuntimes = make(chan *goja.Runtime, 2)

func acquireSearchRuntime(resource map[string]interface{}) (*goja.Runtime, func(), error) {
	searchPrograms.Do(func() {
		if len(fhirPathJs) == 0 {
			searchPrograms.err = fmt.Errorf("fhirPathJs script is empty")
			return
		}
		searchPrograms.fhir, searchPrograms.err = goja.Compile("fhirpath.min.js", fhirPathJs, true)
		if searchPrograms.err == nil {
			searchPrograms.extractor, searchPrograms.err = goja.Compile("searchParameterExtractor.js", searchParameterExtractorJs, true)
		}
	})
	if searchPrograms.err != nil {
		return nil, nil, searchPrograms.err
	}

	var vm *goja.Runtime
	select {
	case vm = <-idleSearchRuntimes:
	default:
		vm = goja.New()
		if err := vm.Set("window", vm.NewObject()); err != nil {
			return nil, nil, err
		}
		for _, program := range []*goja.Program{searchPrograms.fhir, searchPrograms.extractor} {
			if _, err := vm.RunProgram(program); err != nil {
				return nil, nil, err
			}
		}
	}
	if err := vm.Set("fhirResource", resource); err != nil {
		return nil, nil, err
	}
	return vm, func() {
		// Do not keep the last patient's resource in the idle runtime.
		if err := vm.Set("fhirResource", goja.Undefined()); err != nil {
			return
		}
		select {
		case idleSearchRuntimes <- vm:
		default:
		}
	}, nil
}
