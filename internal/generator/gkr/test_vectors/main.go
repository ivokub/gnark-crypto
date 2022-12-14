// Copyright 2020 ConsenSys Software Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by consensys/gnark-crypto DO NOT EDIT

package main

import (
	"encoding/json"
	"fmt"
	fiatshamir "github.com/consensys/gnark-crypto/fiat-shamir"
	"github.com/consensys/gnark-crypto/internal/generator/test_vector_utils/small_rational"
	"github.com/consensys/gnark-crypto/internal/generator/test_vector_utils/small_rational/gkr"
	"github.com/consensys/gnark-crypto/internal/generator/test_vector_utils/small_rational/polynomial"
	"github.com/consensys/gnark-crypto/internal/generator/test_vector_utils/small_rational/sumcheck"
	"github.com/consensys/gnark-crypto/internal/generator/test_vector_utils/small_rational/test_vector_utils"
	"os"
	"path/filepath"
	"reflect"
)

func main() {
	if err := func() error {
		if err := GenerateVectors(); err != nil {
			return err
		}
		return test_vector_utils.SaveUsedHashEntries()
	}(); err != nil {
		fmt.Println(err.Error())
		os.Exit(-1)
	}
}

func GenerateVectors() error {
	testDirPath, err := filepath.Abs("gkr/test_vectors")
	if err != nil {
		return err
	}

	fmt.Printf("generating GKR test cases: scanning directory %s for test specs\n", testDirPath)

	dirEntries, err := os.ReadDir(testDirPath)
	if err != nil {
		return err
	}
	for _, dirEntry := range dirEntries {
		if !dirEntry.IsDir() {

			if filepath.Ext(dirEntry.Name()) == ".json" {

				fmt.Println("\tprocessing", dirEntry.Name())

				path := filepath.Join(testDirPath, dirEntry.Name())

				var testCase *TestCase
				testCase, err = newTestCase(path)
				if err != nil {
					return err
				}

				var proof gkr.Proof
				proof, err = gkr.Prove(testCase.Circuit, testCase.FullAssignment, testCase.transcriptSetting())
				if err != nil {
					return err
				}

				if testCase.Info.Proof, err = toPrintableProof(proof); err != nil {
					return err
				}
				var outBytes []byte
				if outBytes, err = json.MarshalIndent(testCase.Info, "", "\t"); err == nil {
					if err = os.WriteFile(path, outBytes, 0); err != nil {
						return err
					}
				} else {
					return err
				}

				testCase, err = newTestCase(path)
				if err != nil {
					return err
				}

				err = gkr.Verify(testCase.Circuit, testCase.InOutAssignment, proof, testCase.transcriptSetting())
				if err != nil {
					return err
				}

				testCase, err = newTestCase(path)
				if err != nil {
					return err
				}

				err = gkr.Verify(testCase.Circuit, testCase.InOutAssignment, proof, testCase.transcriptSetting())
				if err == nil {
					return fmt.Errorf("bad proof accepted")
				}
			}
		}
	}

	return nil
}

func toPrintableProof(proof gkr.Proof) (PrintableProof, error) {
	res := make(PrintableProof, len(proof))

	for i := range proof {

		partialSumPolys := make([][]interface{}, len(proof[i].PartialSumPolys))
		for k, partialK := range proof[i].PartialSumPolys {
			partialSumPolys[k] = test_vector_utils.ElementSliceToInterfaceSlice(partialK)
		}

		res[i] = PrintableSumcheckProof{
			FinalEvalProof:  test_vector_utils.ElementSliceToInterfaceSlice(proof[i].FinalEvalProof),
			PartialSumPolys: partialSumPolys,
		}
	}
	return res, nil
}

type WireInfo struct {
	Gate   string `json:"gate"`
	Inputs []int  `json:"inputs"`
}

type CircuitInfo []WireInfo

var circuitCache = make(map[string]gkr.Circuit)

func getCircuit(path string) (gkr.Circuit, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if circuit, ok := circuitCache[path]; ok {
		return circuit, nil
	}
	var bytes []byte
	if bytes, err = os.ReadFile(path); err == nil {
		var circuitInfo CircuitInfo
		if err = json.Unmarshal(bytes, &circuitInfo); err == nil {
			circuit := circuitInfo.toCircuit()
			circuitCache[path] = circuit
			return circuit, nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (c CircuitInfo) toCircuit() (circuit gkr.Circuit) {
	circuit = make(gkr.Circuit, len(c))
	for i := range c {
		circuit[i].Gate = gates[c[i].Gate]
		circuit[i].Inputs = make([]*gkr.Wire, len(c[i].Inputs))
		for k, inputCoord := range c[i].Inputs {
			input := &circuit[inputCoord]
			circuit[i].Inputs[k] = input
		}
	}
	return
}

var gates map[string]gkr.Gate

func init() {
	gates = make(map[string]gkr.Gate)
	gates["identity"] = gkr.IdentityGate{}
	gates["mul"] = mulGate{}
	gates["mimc"] = mimcCipherGate{} //TODO: Add ark
}

type mimcCipherGate struct {
	ark small_rational.SmallRational
}

func (m mimcCipherGate) Evaluate(input ...small_rational.SmallRational) (res small_rational.SmallRational) {
	var sum small_rational.SmallRational

	sum.
		Add(&input[0], &input[1]).
		Add(&sum, &m.ark)

	res.Square(&sum)    // sum^2
	res.Mul(&res, &sum) // sum^3
	res.Square(&res)    //sum^6
	res.Mul(&res, &sum) //sum^7

	return
}

func (m mimcCipherGate) Degree() int {
	return 7
}

type PrintableProof []PrintableSumcheckProof

type PrintableSumcheckProof struct {
	FinalEvalProof  interface{}     `json:"finalEvalProof"`
	PartialSumPolys [][]interface{} `json:"partialSumPolys"`
}

func unmarshalProof(printable PrintableProof) (gkr.Proof, error) {
	proof := make(gkr.Proof, len(printable))
	for i := range printable {
		finalEvalProof := []small_rational.SmallRational(nil)

		if printable[i].FinalEvalProof != nil {
			finalEvalSlice := reflect.ValueOf(printable[i].FinalEvalProof)
			finalEvalProof = make([]small_rational.SmallRational, finalEvalSlice.Len())
			for k := range finalEvalProof {
				if _, err := finalEvalProof[k].SetInterface(finalEvalSlice.Index(k).Interface()); err != nil {
					return nil, err
				}
			}
		}

		proof[i] = sumcheck.Proof{
			PartialSumPolys: make([]polynomial.Polynomial, len(printable[i].PartialSumPolys)),
			FinalEvalProof:  finalEvalProof,
		}
		for k := range printable[i].PartialSumPolys {
			var err error
			if proof[i].PartialSumPolys[k], err = test_vector_utils.SliceToElementSlice(printable[i].PartialSumPolys[k]); err != nil {
				return nil, err
			}
		}
	}
	return proof, nil
}

type TestCase struct {
	Circuit         gkr.Circuit
	Hash            *test_vector_utils.ElementMap
	Proof           gkr.Proof
	FullAssignment  gkr.WireAssignment
	InOutAssignment gkr.WireAssignment
	Info            TestCaseInfo
}

type TestCaseInfo struct {
	Hash    string          `json:"hash"`
	Circuit string          `json:"circuit"`
	Input   [][]interface{} `json:"input"`
	Output  [][]interface{} `json:"output"`
	Proof   PrintableProof  `json:"proof"`
}

var testCases = make(map[string]*TestCase)

func newTestCase(path string) (*TestCase, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(path)

	tCase, ok := testCases[path]
	if !ok {
		var bytes []byte
		if bytes, err = os.ReadFile(path); err == nil {
			var info TestCaseInfo
			err = json.Unmarshal(bytes, &info)
			if err != nil {
				return nil, err
			}

			var circuit gkr.Circuit
			if circuit, err = getCircuit(filepath.Join(dir, info.Circuit)); err != nil {
				return nil, err
			}
			var _hash *test_vector_utils.ElementMap
			if _hash, err = test_vector_utils.ElementMapFromFile(filepath.Join(dir, info.Hash)); err != nil {
				return nil, err
			}
			var proof gkr.Proof
			if proof, err = unmarshalProof(info.Proof); err != nil {
				return nil, err
			}

			fullAssignment := make(gkr.WireAssignment)
			inOutAssignment := make(gkr.WireAssignment)

			sorted := gkr.TopologicalSort(circuit)

			inI, outI := 0, 0
			for _, w := range sorted {
				var assignmentRaw []interface{}
				if w.IsInput() {
					if inI == len(info.Input) {
						return nil, fmt.Errorf("fewer input in vector than in circuit")
					}
					assignmentRaw = info.Input[inI]
					inI++
				} else if w.IsOutput() {
					if outI == len(info.Output) {
						return nil, fmt.Errorf("fewer output in vector than in circuit")
					}
					assignmentRaw = info.Output[outI]
					outI++
				}
				if assignmentRaw != nil {
					var wireAssignment []small_rational.SmallRational
					if wireAssignment, err = test_vector_utils.SliceToElementSlice(assignmentRaw); err != nil {
						return nil, err
					}

					fullAssignment[w] = wireAssignment
					inOutAssignment[w] = wireAssignment
				}
			}

			fullAssignment.Complete(circuit)

			info.Output = make([][]interface{}, 0, outI)

			for _, w := range sorted {
				if w.IsOutput() {

					info.Output = append(info.Output, test_vector_utils.ElementSliceToInterfaceSlice(inOutAssignment[w]))

				}
			}

			tCase = &TestCase{
				FullAssignment:  fullAssignment,
				InOutAssignment: inOutAssignment,
				Proof:           proof,
				Hash:            _hash,
				Circuit:         circuit,
				Info:            info,
			}

			testCases[path] = tCase
		} else {
			return nil, err
		}
	}

	return tCase, nil
}

func (c *TestCase) transcriptSetting() fiatshamir.Settings {
	return fiatshamir.WithHash(&test_vector_utils.MapHash{Map: c.Hash})
}

type mulGate struct{}

func (g mulGate) Evaluate(element ...small_rational.SmallRational) (result small_rational.SmallRational) {
	result.Mul(&element[0], &element[1])
	return
}

func (g mulGate) Degree() int {
	return 2
}
