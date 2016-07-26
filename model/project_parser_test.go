package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/evergreen-ci/evergreen/command"
	"github.com/evergreen-ci/evergreen/util"
	. "github.com/smartystreets/goconvey/convey"
)

// ShouldContainResembling tests whether a slice contains an element that DeepEquals
// the expected input. TODO make this a subpkg
func ShouldContainResembling(actual interface{}, expected ...interface{}) string {
	if len(expected) != 1 {
		return "ShouldContainResembling takes 1 argument"
	}
	if !util.SliceContains(actual, expected[0]) {
		return fmt.Sprintf("%#v does not contain %#v", actual, expected[0])
	}
	return ""
}

func TestCreateIntermediateProjectDependencies(t *testing.T) {
	Convey("Testing different project files", t, func() {
		Convey("a simple project file should parse", func() {
			simple := `
tasks:
- name: "compile"
- name: task0
- name: task1
  patchable: false
  tags: ["tag1", "tag2"]
  depends_on:
  - compile
  - name: "task0"
    status: "failed"
    patch_optional: true
`
			p, errs := createIntermediateProject([]byte(simple))
			So(p, ShouldNotBeNil)
			So(len(errs), ShouldEqual, 0)
			So(p.Tasks[2].DependsOn[0].Name, ShouldEqual, "compile")
			So(p.Tasks[2].DependsOn[0].PatchOptional, ShouldEqual, false)
			So(p.Tasks[2].DependsOn[1].Name, ShouldEqual, "task0")
			So(p.Tasks[2].DependsOn[1].Status, ShouldEqual, "failed")
			So(p.Tasks[2].DependsOn[1].PatchOptional, ShouldEqual, true)
		})
		Convey("a file with a single dependency should parse", func() {
			single := `
tasks:
- name: "compile"
- name: task0
- name: task1
  depends_on: task0
`
			p, errs := createIntermediateProject([]byte(single))
			So(p, ShouldNotBeNil)
			So(len(errs), ShouldEqual, 0)
			So(p.Tasks[2].DependsOn[0].Name, ShouldEqual, "task0")
		})
		Convey("a file with a nameless dependency should error", func() {
			Convey("with a single dep", func() {
				nameless := `
tasks:
- name: "compile"
  depends_on: ""
`
				p, errs := createIntermediateProject([]byte(nameless))
				So(p, ShouldBeNil)
				So(len(errs), ShouldEqual, 1)
			})
			Convey("or multiple", func() {
				nameless := `
tasks:
- name: "compile"
  depends_on:
  - name: "task1"
  - status: "failed" #this has no task attached
`
				p, errs := createIntermediateProject([]byte(nameless))
				So(p, ShouldBeNil)
				So(len(errs), ShouldEqual, 1)
			})
			Convey("but an unused depends_on field should not error", func() {
				nameless := `
tasks:
- name: "compile"
`
				p, errs := createIntermediateProject([]byte(nameless))
				So(p, ShouldNotBeNil)
				So(len(errs), ShouldEqual, 0)
			})
		})
	})
}

func TestCreateIntermediateProjectRequirements(t *testing.T) {
	Convey("Testing different project files", t, func() {
		Convey("a simple project file should parse", func() {
			simple := `
tasks:
- name: task0
- name: task1
  requires:
  - name: "task0"
    variant: "v1"
  - "task2"
`
			p, errs := createIntermediateProject([]byte(simple))
			So(p, ShouldNotBeNil)
			So(len(errs), ShouldEqual, 0)
			So(p.Tasks[1].Requires[0].Name, ShouldEqual, "task0")
			So(p.Tasks[1].Requires[0].Variant.stringSelector, ShouldEqual, "v1")
			So(p.Tasks[1].Requires[1].Name, ShouldEqual, "task2")
			So(p.Tasks[1].Requires[1].Variant, ShouldBeNil)
		})
		Convey("a single requirement should parse", func() {
			simple := `
tasks:
- name: task1
  requires:
    name: "task0"
    variant: "v1"
`
			p, errs := createIntermediateProject([]byte(simple))
			So(p, ShouldNotBeNil)
			So(len(errs), ShouldEqual, 0)
			So(p.Tasks[0].Requires[0].Name, ShouldEqual, "task0")
			So(p.Tasks[0].Requires[0].Variant.stringSelector, ShouldEqual, "v1")
		})
		Convey("a single requirement with a matrix selector should parse", func() {
			simple := `
tasks:
- name: task1
  requires:
    name: "task0"
    variant:
     cool: "shoes"
     colors:
      - red
      - green
      - blue
`
			p, errs := createIntermediateProject([]byte(simple))
			So(errs, ShouldBeNil)
			So(p, ShouldNotBeNil)
			So(p.Tasks[0].Requires[0].Name, ShouldEqual, "task0")
			So(p.Tasks[0].Requires[0].Variant.stringSelector, ShouldEqual, "")
			So(p.Tasks[0].Requires[0].Variant.matrixSelector, ShouldResemble, matrixDefinition{
				"cool": []string{"shoes"}, "colors": []string{"red", "green", "blue"},
			})
		})
	})
}

func TestCreateIntermediateProjectBuildVariants(t *testing.T) {
	Convey("Testing different project files", t, func() {
		Convey("a file with multiple BVTs should parse", func() {
			simple := `
buildvariants:
- name: "v1"
  stepback: true
  batchtime: 123
  modules: ["wow","cool"]
  run_on:
  - "windows2000"
  tasks:
  - name: "t1"
  - name: "t2"
    depends_on:
    - name: "t3"
      variant: "v0"
    requires:
    - name: "t4"
    stepback: false
    priority: 77
`
			p, errs := createIntermediateProject([]byte(simple))
			So(p, ShouldNotBeNil)
			So(len(errs), ShouldEqual, 0)
			bv := p.BuildVariants[0]
			So(bv.Name, ShouldEqual, "v1")
			So(*bv.Stepback, ShouldBeTrue)
			So(bv.RunOn[0], ShouldEqual, "windows2000")
			So(len(bv.Modules), ShouldEqual, 2)
			So(bv.Tasks[0].Name, ShouldEqual, "t1")
			So(bv.Tasks[1].Name, ShouldEqual, "t2")
			So(bv.Tasks[1].DependsOn[0].TaskSelector, ShouldResemble,
				TaskSelector{Name: "t3", Variant: &variantSelector{stringSelector: "v0"}})
			So(bv.Tasks[1].Requires[0], ShouldResemble, TaskSelector{Name: "t4"})
			So(*bv.Tasks[1].Stepback, ShouldBeFalse)
			So(bv.Tasks[1].Priority, ShouldEqual, 77)
		})
		Convey("a file with oneline BVTs should parse", func() {
			simple := `
buildvariants:
- name: "v1"
  tasks:
  - "t1"
  - name: "t2"
    depends_on: "t3"
    requires: "t4"
`
			p, errs := createIntermediateProject([]byte(simple))
			So(p, ShouldNotBeNil)
			So(len(errs), ShouldEqual, 0)
			bv := p.BuildVariants[0]
			So(bv.Name, ShouldEqual, "v1")
			So(bv.Tasks[0].Name, ShouldEqual, "t1")
			So(bv.Tasks[1].Name, ShouldEqual, "t2")
			So(bv.Tasks[1].DependsOn[0].TaskSelector, ShouldResemble, TaskSelector{Name: "t3"})
			So(bv.Tasks[1].Requires[0], ShouldResemble, TaskSelector{Name: "t4"})
		})
		Convey("a file with single BVTs should parse", func() {
			simple := `
buildvariants:
- name: "v1"
  tasks: "*"
- name: "v2"
  tasks:
    name: "t1"
`
			p, errs := createIntermediateProject([]byte(simple))
			So(p, ShouldNotBeNil)
			So(len(errs), ShouldEqual, 0)
			So(len(p.BuildVariants), ShouldEqual, 2)
			bv1 := p.BuildVariants[0]
			bv2 := p.BuildVariants[1]
			So(bv1.Name, ShouldEqual, "v1")
			So(bv2.Name, ShouldEqual, "v2")
			So(len(bv1.Tasks), ShouldEqual, 1)
			So(bv1.Tasks[0].Name, ShouldEqual, "*")
			So(len(bv2.Tasks), ShouldEqual, 1)
			So(bv2.Tasks[0].Name, ShouldEqual, "t1")
		})
		Convey("a file with single run_on, tags, and ignore fields should parse ", func() {
			single := `
ignore: "*.md"
tasks:
- name: "t1"
  tags: wow
buildvariants:
- name: "v1"
  run_on: "distro1"
  tasks: "*"
`
			p, errs := createIntermediateProject([]byte(single))
			So(p, ShouldNotBeNil)
			So(len(errs), ShouldEqual, 0)
			So(len(p.Ignore), ShouldEqual, 1)
			So(p.Ignore[0], ShouldEqual, "*.md")
			So(len(p.Tasks[0].Tags), ShouldEqual, 1)
			So(p.Tasks[0].Tags[0], ShouldEqual, "wow")
			So(len(p.BuildVariants), ShouldEqual, 1)
			bv1 := p.BuildVariants[0]
			So(bv1.Name, ShouldEqual, "v1")
			So(len(bv1.RunOn), ShouldEqual, 1)
			So(bv1.RunOn[0], ShouldEqual, "distro1")
		})
		Convey("a file that uses run_on for BVTasks should parse", func() {
			single := `
buildvariants:
- name: "v1"
  tasks:
  - name: "t1"
    run_on: "test"
`
			p, errs := createIntermediateProject([]byte(single))
			So(p, ShouldNotBeNil)
			So(len(errs), ShouldEqual, 0)
			So(p.BuildVariants[0].Tasks[0].Distros[0], ShouldEqual, "test")
			So(p.BuildVariants[0].Tasks[0].RunOn, ShouldBeNil)
		})
		Convey("a file that uses run_on AND distros for BVTasks should not parse", func() {
			single := `
buildvariants:
- name: "v1"
  tasks:
  - name: "t1"
    run_on: "test"
    distros: "asdasdasd"
`
			p, errs := createIntermediateProject([]byte(single))
			So(p, ShouldBeNil)
			So(len(errs), ShouldEqual, 1)
		})
	})
}

func TestTranslateDependsOn(t *testing.T) {
	Convey("With an intermediate parseProject", t, func() {
		pp := &parserProject{}
		Convey("a tag-free dependency config should be unchanged", func() {
			pp.BuildVariants = []parserBV{
				{Name: "v1"},
			}
			pp.Tasks = []parserTask{
				{Name: "t1"},
				{Name: "t2"},
				{Name: "t3", DependsOn: parserDependencies{
					{TaskSelector: TaskSelector{Name: "t1"}},
					{TaskSelector: TaskSelector{
						Name: "t2", Variant: &variantSelector{stringSelector: "v1"}}}},
				},
			}
			out, errs := translateProject(pp)
			So(out, ShouldNotBeNil)
			So(len(errs), ShouldEqual, 0)
			deps := out.Tasks[2].DependsOn
			So(deps[0].Name, ShouldEqual, "t1")
			So(deps[1].Name, ShouldEqual, "t2")
			So(deps[1].Variant, ShouldEqual, "v1")
		})
		Convey("a dependency with tag selectors should evaluate", func() {
			pp.BuildVariants = []parserBV{
				{Name: "v1", Tags: []string{"cool"}},
				{Name: "v2", Tags: []string{"cool"}},
			}
			pp.Tasks = []parserTask{
				{Name: "t1", Tags: []string{"a", "b"}},
				{Name: "t2", Tags: []string{"a", "c"}, DependsOn: parserDependencies{
					{TaskSelector: TaskSelector{Name: "*"}}}},
				{Name: "t3", DependsOn: parserDependencies{
					{TaskSelector: TaskSelector{
						Name: ".b", Variant: &variantSelector{stringSelector: ".cool !v2"}}},
					{TaskSelector: TaskSelector{
						Name: ".a !.b", Variant: &variantSelector{stringSelector: ".cool"}}}},
				},
			}
			out, errs := translateProject(pp)
			So(out, ShouldNotBeNil)
			So(len(errs), ShouldEqual, 0)
			So(out.Tasks[1].DependsOn[0].Name, ShouldEqual, "*")
			deps := out.Tasks[2].DependsOn
			So(deps[0].Name, ShouldEqual, "t1")
			So(deps[0].Variant, ShouldEqual, "v1")
			So(deps[1].Name, ShouldEqual, "t2")
			So(deps[1].Variant, ShouldEqual, "v1")
			So(deps[2].Name, ShouldEqual, "t2")
			So(deps[2].Variant, ShouldEqual, "v2")
		})
		Convey("a dependency with erroneous selectors should fail", func() {
			pp.BuildVariants = []parserBV{
				{Name: "v1"},
			}
			pp.Tasks = []parserTask{
				{Name: "t1", Tags: []string{"a", "b"}},
				{Name: "t2", Tags: []string{"a", "c"}},
				{Name: "t3", DependsOn: parserDependencies{
					{TaskSelector: TaskSelector{Name: ".cool"}},
					{TaskSelector: TaskSelector{Name: "!!.cool"}},                                                  //[1] illegal selector
					{TaskSelector: TaskSelector{Name: "!.c !.b", Variant: &variantSelector{stringSelector: "v1"}}}, //[2] no matching tasks
					{TaskSelector: TaskSelector{Name: "t1", Variant: &variantSelector{stringSelector: ".nope"}}},   //[3] no matching variants
					{TaskSelector: TaskSelector{Name: "t1"}, Status: "*"},                                          // valid, but:
					{TaskSelector: TaskSelector{Name: ".b"}},                                                       //[4] conflicts with above
				}},
			}
			out, errs := translateProject(pp)
			So(out, ShouldNotBeNil)
			So(len(errs), ShouldEqual, 4)
		})
	})
}

func TestTranslateRequires(t *testing.T) {
	Convey("With an intermediate parseProject", t, func() {
		pp := &parserProject{}
		Convey("a task with valid requirements should succeed", func() {
			pp.BuildVariants = []parserBV{
				{Name: "v1"},
			}
			pp.Tasks = []parserTask{
				{Name: "t1"},
				{Name: "t2"},
				{Name: "t3", Requires: TaskSelectors{
					{Name: "t1"},
					{Name: "t2", Variant: &variantSelector{stringSelector: "v1"}},
				}},
			}
			out, errs := translateProject(pp)
			So(out, ShouldNotBeNil)
			So(len(errs), ShouldEqual, 0)
			reqs := out.Tasks[2].Requires
			So(reqs[0].Name, ShouldEqual, "t1")
			So(reqs[1].Name, ShouldEqual, "t2")
			So(reqs[1].Variant, ShouldEqual, "v1")
		})
		Convey("a task with erroneous requirements should fail", func() {
			pp.BuildVariants = []parserBV{
				{Name: "v1"},
			}
			pp.Tasks = []parserTask{
				{Name: "t1"},
				{Name: "t2", Tags: []string{"taggy"}},
				{Name: "t3", Requires: TaskSelectors{
					{Name: "!!!!!"}, //illegal selector
					{Name: ".taggy !t2", Variant: &variantSelector{stringSelector: "v1"}}, //nothing returned
					{Name: "t1", Variant: &variantSelector{stringSelector: "!v1"}},        //no variants returned
					{Name: "t1 t2"}, //nothing returned
				}},
			}
			out, errs := translateProject(pp)
			So(out, ShouldNotBeNil)
			So(len(errs), ShouldEqual, 4)
		})
	})
}

func TestTranslateBuildVariants(t *testing.T) {
	Convey("With an intermediate parseProject", t, func() {
		pp := &parserProject{}
		Convey("a project with valid variant tasks should succeed", func() {
			pp.Tasks = []parserTask{
				{Name: "t1"},
				{Name: "t2", Tags: []string{"a", "z"}},
				{Name: "t3", Tags: []string{"a", "b"}},
			}
			pp.BuildVariants = []parserBV{{
				Name: "v1",
				Tasks: parserBVTasks{
					{Name: "t1"},
					{Name: ".z", DependsOn: parserDependencies{
						{TaskSelector: TaskSelector{Name: ".b"}}}},
					{Name: "* !t1 !t2", Requires: TaskSelectors{{Name: "!.a"}}},
				},
			}}

			out, errs := translateProject(pp)
			So(out, ShouldNotBeNil)
			So(len(errs), ShouldEqual, 0)
			bvts := out.BuildVariants[0].Tasks
			So(bvts[0].Name, ShouldEqual, "t1")
			So(bvts[1].Name, ShouldEqual, "t2")
			So(bvts[2].Name, ShouldEqual, "t3")
			So(bvts[1].DependsOn[0].Name, ShouldEqual, "t3")
			So(bvts[2].Requires[0].Name, ShouldEqual, "t1")
		})
		Convey("a bvtask with erroneous requirements should fail", func() {
			pp.Tasks = []parserTask{
				{Name: "t1"},
			}
			pp.BuildVariants = []parserBV{{
				Name: "v1",
				Tasks: parserBVTasks{
					{Name: "t1", Requires: TaskSelectors{{Name: ".b"}}},
				},
			}}
			out, errs := translateProject(pp)
			So(out, ShouldNotBeNil)
			So(len(errs), ShouldEqual, 1)
		})
	})
}

func parserTaskSelectorTaskEval(tse *taskSelectorEvaluator, tasks parserBVTasks, expected []BuildVariantTask) {
	names := []string{}
	exp := []string{}
	for _, t := range tasks {
		names = append(names, t.Name)
	}
	for _, e := range expected {
		exp = append(exp, e.Name)
	}
	vse := NewVariantSelectorEvaluator([]parserBV{}, nil)
	Convey(fmt.Sprintf("tasks [%v] should evaluate to [%v]",
		strings.Join(names, ", "), strings.Join(exp, ", ")), func() {
		ts, errs := evaluateBVTasks(tse, vse, tasks)
		if expected != nil {
			So(errs, ShouldBeNil)
		} else {
			So(errs, ShouldNotBeNil)
		}
		So(len(ts), ShouldEqual, len(expected))
		for _, e := range expected {
			exists := false
			for _, t := range ts {
				if t.Name == e.Name && t.Priority == e.Priority && len(t.DependsOn) == len(e.DependsOn) {
					exists = true
				}
			}
			So(exists, ShouldBeTrue)
		}
	})
}

func TestParserTaskSelectorEvaluation(t *testing.T) {
	Convey("With a colorful set of ProjectTasks", t, func() {
		taskDefs := []parserTask{
			{Name: "red", Tags: []string{"primary", "warm"}},
			{Name: "orange", Tags: []string{"secondary", "warm"}},
			{Name: "yellow", Tags: []string{"primary", "warm"}},
			{Name: "green", Tags: []string{"secondary", "cool"}},
			{Name: "blue", Tags: []string{"primary", "cool"}},
			{Name: "purple", Tags: []string{"secondary", "cool"}},
			{Name: "brown", Tags: []string{"tertiary"}},
			{Name: "black", Tags: []string{"special"}},
			{Name: "white", Tags: []string{"special"}},
		}

		Convey("a project parser", func() {
			tse := NewParserTaskSelectorEvaluator(taskDefs)
			Convey("should evaluate valid tasks pointers properly", func() {
				parserTaskSelectorTaskEval(tse,
					parserBVTasks{{Name: "white"}},
					[]BuildVariantTask{{Name: "white"}})
				parserTaskSelectorTaskEval(tse,
					parserBVTasks{{Name: "red", Priority: 500}, {Name: ".secondary"}},
					[]BuildVariantTask{{Name: "red", Priority: 500}, {Name: "orange"}, {Name: "purple"}, {Name: "green"}})
				parserTaskSelectorTaskEval(tse,
					parserBVTasks{
						{Name: "orange", Distros: []string{"d1"}},
						{Name: ".warm .secondary", Distros: []string{"d1"}}},
					[]BuildVariantTask{{Name: "orange", Distros: []string{"d1"}}})
				parserTaskSelectorTaskEval(tse,
					parserBVTasks{
						{Name: "orange", Distros: []string{"d1"}},
						{Name: "!.warm .secondary", Distros: []string{"d1"}}},
					[]BuildVariantTask{
						{Name: "orange", Distros: []string{"d1"}},
						{Name: "purple", Distros: []string{"d1"}},
						{Name: "green", Distros: []string{"d1"}}})
				parserTaskSelectorTaskEval(tse,
					parserBVTasks{{Name: "*"}},
					[]BuildVariantTask{
						{Name: "red"}, {Name: "blue"}, {Name: "yellow"},
						{Name: "orange"}, {Name: "purple"}, {Name: "green"},
						{Name: "brown"}, {Name: "white"}, {Name: "black"},
					})
				parserTaskSelectorTaskEval(tse,
					parserBVTasks{
						{Name: "red", Priority: 100},
						{Name: "!.warm .secondary", Priority: 100}},
					[]BuildVariantTask{
						{Name: "red", Priority: 100},
						{Name: "purple", Priority: 100},
						{Name: "green", Priority: 100}})
			})
		})
	})
}

func TestMatrixIntermediateParsing(t *testing.T) {
	Convey("Testing different project files with matrix definitions", t, func() {
		Convey("a set of axes should parse", func() {
			axes := `
axes:
- id: os
  display_name: Operating System
  values:
  - id: ubuntu
    display_name: Ubuntu
    tags: "linux"
    variables:
      user: root
    run_on: ubuntu_small
  - id: rhel
    display_name: Red Hat
    tags: ["linux", "enterprise"]
    run_on:
    - rhel55
    - rhel62
`
			p, errs := createIntermediateProject([]byte(axes))
			So(errs, ShouldBeNil)
			axis := p.Axes[0]
			So(axis.Id, ShouldEqual, "os")
			So(axis.DisplayName, ShouldEqual, "Operating System")
			So(len(axis.Values), ShouldEqual, 2)
			So(axis.Values[0], ShouldResemble, axisValue{
				Id:          "ubuntu",
				DisplayName: "Ubuntu",
				Tags:        []string{"linux"},
				Variables:   map[string]string{"user": "root"},
				RunOn:       []string{"ubuntu_small"},
			})
			So(axis.Values[1], ShouldResemble, axisValue{
				Id:          "rhel",
				DisplayName: "Red Hat",
				Tags:        []string{"linux", "enterprise"},
				RunOn:       []string{"rhel55", "rhel62"},
			})
		})
		Convey("a barebones matrix definition should parse", func() {
			simple := `
buildvariants:
- matrix_name: "test"
  matrix_spec: {"os": ".linux", "bits":["32", "64"]}
  exclude_spec: [{"os":"ubuntu", "bits":"32"}]
- matrix_name: "test2"
  matrix_spec:
    os: "windows95"
    color:
    - red
    - blue
    - green
`
			p, errs := createIntermediateProject([]byte(simple))
			So(errs, ShouldBeNil)
			So(len(p.matrices), ShouldEqual, 2)
			m1 := p.matrices[0]
			So(m1, ShouldResemble, matrix{
				Id: "test",
				Spec: matrixDefinition{
					"os":   []string{".linux"},
					"bits": []string{"32", "64"},
				},
				Exclude: []matrixDefinition{
					{"os": []string{"ubuntu"}, "bits": []string{"32"}},
				},
			})
			m2 := p.matrices[1]
			So(m2, ShouldResemble, matrix{
				Id: "test2",
				Spec: matrixDefinition{
					"os":    []string{"windows95"},
					"color": []string{"red", "blue", "green"},
				},
			})
		})
		Convey("a mixed definition should parse", func() {
			simple := `
buildvariants:
- matrix_name: "test"
  matrix_spec: {"os": "*", "bits": "*"}
- name: "single_variant"
  tasks: "*"
`
			p, errs := createIntermediateProject([]byte(simple))
			So(errs, ShouldBeNil)
			So(len(p.matrices), ShouldEqual, 1)
			m1 := p.matrices[0]
			So(m1.Id, ShouldEqual, "test")
			So(len(p.BuildVariants), ShouldEqual, 1)
			So(p.BuildVariants[0].Name, ShouldEqual, "single_variant")
			So(p.BuildVariants[0].Tasks, ShouldResemble, parserBVTasks{parserBVTask{Name: "*"}})
		})
	})
}

func TestMatrixDefinitionAllCells(t *testing.T) {
	Convey("With a set of test definitions", t, func() {
		Convey("an empty definition should return an empty list", func() {
			a := matrixDefinition{}
			cells := a.allCells()
			So(len(cells), ShouldEqual, 0)
		})
		Convey("an empty axis should cause a panic", func() {
			a := matrixDefinition{
				"a": []string{},
				"b": []string{"1"},
			}
			So(func() { a.allCells() }, ShouldPanic)
		})
		Convey("a one-cell matrix should return a one-item list", func() {
			a := matrixDefinition{
				"a": []string{"0"},
			}
			cells := a.allCells()
			So(len(cells), ShouldEqual, 1)
			So(cells, ShouldContainResembling, matrixValue{"a": "0"})
			b := matrixDefinition{
				"a": []string{"0"},
				"b": []string{"1"},
				"c": []string{"2"},
			}
			cells = b.allCells()
			So(len(cells), ShouldEqual, 1)
			So(cells, ShouldContainResembling, matrixValue{"a": "0", "b": "1", "c": "2"})
		})
		Convey("a one-axis matrix should return an equivalent list", func() {
			a := matrixDefinition{
				"a": []string{"0", "1", "2"},
			}
			cells := a.allCells()
			So(len(cells), ShouldEqual, 3)
			So(cells, ShouldContainResembling, matrixValue{"a": "0"})
			So(cells, ShouldContainResembling, matrixValue{"a": "1"})
			So(cells, ShouldContainResembling, matrixValue{"a": "2"})
			b := matrixDefinition{
				"a": []string{"0"},
				"b": []string{"0", "1", "2"},
			}
			cells = b.allCells()
			So(len(cells), ShouldEqual, 3)
			So(cells, ShouldContainResembling, matrixValue{"b": "0", "a": "0"})
			So(cells, ShouldContainResembling, matrixValue{"b": "1", "a": "0"})
			So(cells, ShouldContainResembling, matrixValue{"b": "2", "a": "0"})
			c := matrixDefinition{
				"c": []string{"0", "1", "2"},
				"d": []string{"0"},
			}
			cells = c.allCells()
			So(len(cells), ShouldEqual, 3)
			So(cells, ShouldContainResembling, matrixValue{"c": "0", "d": "0"})
			So(cells, ShouldContainResembling, matrixValue{"c": "1", "d": "0"})
			So(cells, ShouldContainResembling, matrixValue{"c": "2", "d": "0"})
		})
		Convey("a 2x2 matrix should expand properly", func() {
			a := matrixDefinition{
				"a": []string{"0", "1"},
				"b": []string{"0", "1"},
			}
			cells := a.allCells()
			So(len(cells), ShouldEqual, 4)
			So(cells, ShouldContainResembling, matrixValue{"a": "0", "b": "0"})
			So(cells, ShouldContainResembling, matrixValue{"a": "1", "b": "0"})
			So(cells, ShouldContainResembling, matrixValue{"a": "0", "b": "1"})
			So(cells, ShouldContainResembling, matrixValue{"a": "1", "b": "1"})
		})
		Convey("a disgustingly large matrix should expand properly", func() {
			bigList := func(max int) []string {
				out := []string{}
				for i := 0; i < max; i++ {
					out = append(out, fmt.Sprint(i))
				}
				return out
			}

			huge := matrixDefinition{
				"a": bigList(15),
				"b": bigList(290),
				"c": bigList(20),
			}
			cells := huge.allCells()
			So(len(cells), ShouldEqual, 15*290*20)
			So(cells, ShouldContainResembling, matrixValue{"a": "0", "b": "0", "c": "0"})
			So(cells, ShouldContainResembling, matrixValue{"a": "14", "b": "289", "c": "19"})
			// some random guesses just for fun
			So(cells, ShouldContainResembling, matrixValue{"a": "10", "b": "29", "c": "1"})
			So(cells, ShouldContainResembling, matrixValue{"a": "1", "b": "2", "c": "17"})
			So(cells, ShouldContainResembling, matrixValue{"a": "8", "b": "100", "c": "5"})
		})
	})
}

func TestMatrixDefinitionContains(t *testing.T) {
	Convey("With a set of test definitions", t, func() {
		Convey("an empty definition should match nothing", func() {
			a := matrixDefinition{}
			So(a.contains(matrixValue{"a": "0"}), ShouldBeFalse)
		})
		Convey("all definitions contain the empty value", func() {
			a := matrixDefinition{}
			So(a.contains(matrixValue{}), ShouldBeTrue)
			b := matrixDefinition{
				"a": []string{"0", "1"},
				"b": []string{"0", "1"},
			}
			So(b.contains(matrixValue{}), ShouldBeTrue)
		})
		Convey("a one-axis matrix should match all of its elements", func() {
			a := matrixDefinition{
				"a": []string{"0", "1", "2"},
			}
			So(a.contains(matrixValue{"a": "0"}), ShouldBeTrue)
			So(a.contains(matrixValue{"a": "1"}), ShouldBeTrue)
			So(a.contains(matrixValue{"a": "2"}), ShouldBeTrue)
			So(a.contains(matrixValue{"a": "3"}), ShouldBeFalse)
		})
		Convey("a 2x2 matrix should match all of its elements", func() {
			a := matrixDefinition{
				"a": []string{"0", "1"},
				"b": []string{"0", "1"},
			}
			cells := a.allCells()
			So(len(cells), ShouldEqual, 4)
			So(a.contains(matrixValue{"a": "0", "b": "0"}), ShouldBeTrue)
			So(a.contains(matrixValue{"a": "1", "b": "0"}), ShouldBeTrue)
			So(a.contains(matrixValue{"a": "0", "b": "1"}), ShouldBeTrue)
			So(a.contains(matrixValue{"a": "1", "b": "1"}), ShouldBeTrue)
			So(a.contains(matrixValue{"a": "1", "b": "2"}), ShouldBeFalse)
			Convey("and sub-match all of its individual axis values", func() {
				So(a.contains(matrixValue{"a": "0"}), ShouldBeTrue)
				So(a.contains(matrixValue{"a": "1"}), ShouldBeTrue)
				So(a.contains(matrixValue{"b": "0"}), ShouldBeTrue)
				So(a.contains(matrixValue{"b": "1"}), ShouldBeTrue)
				So(a.contains(matrixValue{"b": "7"}), ShouldBeFalse)
				So(a.contains(matrixValue{"c": "1"}), ShouldBeFalse)
				So(a.contains(matrixValue{"a": "1", "b": "1", "c": "1"}), ShouldBeFalse)
			})
		})
	})
}

func TestBuildMatrixVariantSimple(t *testing.T) {
	testMatrix := &matrix{Id: "test"}
	Convey("With a set of test axes", t, func() {
		axes := []matrixAxis{
			{
				Id: "a",
				Values: []axisValue{
					{Id: "0", Tags: []string{"zero"}},
					{Id: "1", Tags: []string{"odd"}},
					{Id: "2", Tags: []string{"even", "prime"}},
					{Id: "3", Tags: []string{"odd", "prime"}},
				},
			},
			{
				Id: "b",
				Values: []axisValue{
					{Id: "0", Tags: []string{"zero"}},
					{Id: "1", Tags: []string{"odd"}},
					{Id: "2", Tags: []string{"even", "prime"}},
					{Id: "3", Tags: []string{"odd", "prime"}},
				},
			},
		}
		Convey("and matrix value test:{a:0, b:0}", func() {
			mv := matrixValue{"a": "0", "b": "0"}
			Convey("the variant should build without error", func() {
				v, err := buildMatrixVariant(axes, mv, testMatrix, nil)
				So(err, ShouldBeNil)
				Convey("with id='test__a~0_b~0', tags=[zero]", func() {
					So(v.Name, ShouldEqual, "test__a~0_b~0")
					So(v.matrixVal, ShouldResemble, mv)
					So(v.Tags, ShouldContain, "zero")
					So(v.matrixId, ShouldEqual, "test")
				})
			})
		})
		Convey("and matrix value test:{a:1, b:3}", func() {
			mv := matrixValue{"b": "3", "a": "1"}
			Convey("the variant should build without error", func() {
				v, err := buildMatrixVariant(axes, mv, testMatrix, nil)
				So(err, ShouldBeNil)
				Convey("with id='test__a~1_b~3', tags=[odd, prime]", func() {
					So(v.Name, ShouldEqual, "test__a~1_b~3")
					So(v.Tags, ShouldContain, "odd")
					So(v.Tags, ShouldContain, "prime")
				})
			})
		})
		Convey("and a matrix value that references non-existant axis values", func() {
			mv := matrixValue{"b": "2", "a": "4"}
			Convey("should return an error", func() {
				_, err := buildMatrixVariant(axes, mv, testMatrix, nil)
				So(err, ShouldNotBeNil)
			})
		})
		Convey("and a matrix value that references non-existant axis names", func() {
			mv := matrixValue{"b": "2", "coolfun": "4"}
			Convey("should return an error", func() {
				_, err := buildMatrixVariant(axes, mv, testMatrix, nil)
				So(err, ShouldNotBeNil)
			})
		})
	})
}

// helper for pulling variants out of a list
func findVariant(vs []parserBV, id string) parserBV {
	for _, v := range vs {
		if v.Name == id {
			return v
		}
	}
	panic("not found")
}

func TestMatrixVariantsSimple(t *testing.T) {
	Convey("With a delicious set of test axes", t, func() {
		// These tests are structured around a magical project that tests
		// colorful candies. We will be testing M&Ms, Skittles, and Necco Wafers
		// (all candies copyright their respective holders). We need to test
		// each color of each candy individually, so we've decided to simplify
		// our variant definitions with a matrix! The colors are as follows:
		//  M&Ms:     red, orange, yellow, green, blue, brown (6)
		//  Skittles: red, orange, yellow, green, purple (5)
		//  Necco:    orange, yellow, green, purple, pink, brown, black, white (8)
		// TODO: maybe move this up top for multiple tests
		axes := []matrixAxis{
			{
				Id: "color",
				Values: []axisValue{
					{Id: "red", Tags: []string{"hot_color"}},
					{Id: "pink", Tags: []string{"hot_color"}},
					{Id: "orange", Tags: []string{"hot_color"}},
					{Id: "yellow", Tags: []string{"hot_color"}},
					{Id: "brown", Tags: []string{"hot_color"}},
					{Id: "green", Tags: []string{"cool_color"}},
					{Id: "blue", Tags: []string{"cool_color"}},
					{Id: "purple", Tags: []string{"cool_color"}},
					{Id: "black"},
					{Id: "white"},
				},
			},
			{
				Id: "brand",
				Values: []axisValue{
					{Id: "m&ms", Tags: []string{"chocolate"}},
					{Id: "skittles", Tags: []string{"chewy"}},
					{Id: "necco", Tags: []string{"chalk"}},
				},
			},
		}
		ase := NewAxisSelectorEvaluator(axes)
		So(ase, ShouldNotBeNil)
		Convey("and a valid matrix", func() {
			m := matrix{
				Id: "candy",
				Spec: matrixDefinition{
					"color": []string{
						"red", "orange", "yellow", "brown", "green",
						"blue", "purple", "black", "white", "pink",
					},
					"brand": []string{"m&ms", "skittles", "necco"},
				},
				Exclude: []matrixDefinition{
					{"brand": []string{"skittles"}, "color": []string{"brown", "blue"}},
					{"brand": []string{"m&ms"}, "color": []string{"purple"}},
					{"brand": []string{"m&ms", "skittles"},
						"color": []string{"pink", "black", "white"}},
					{"brand": []string{"necco"}, "color": []string{"red", "blue"}},
				},
			}
			Convey("building a list of variants should succeed", func() {
				vs, errs := buildMatrixVariants(axes, ase, []matrix{m})
				So(errs, ShouldBeNil)
				Convey("and return the correct list of combinations", func() {
					So(len(vs), ShouldEqual, 19)
					// check a couple random samples
					d1 := findVariant(vs, "candy__color~yellow_brand~skittles")
					So(d1.Tags, ShouldContain, "hot_color")
					So(d1.Tags, ShouldContain, "chewy")
					d2 := findVariant(vs, "candy__color~black_brand~necco")
					So(len(d2.Tags), ShouldEqual, 1)
					So(d2.Tags, ShouldContain, "chalk")
					// ensure all values are in there...
					vals := []matrixValue{}
					for _, v := range vs {
						vals = append(vals, v.matrixVal)
					}
					So(vals, ShouldContainResembling, matrixValue{"brand": "m&ms", "color": "red"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "m&ms", "color": "orange"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "m&ms", "color": "yellow"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "m&ms", "color": "green"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "m&ms", "color": "blue"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "m&ms", "color": "brown"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "skittles", "color": "red"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "skittles", "color": "orange"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "skittles", "color": "yellow"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "skittles", "color": "green"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "skittles", "color": "purple"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "necco", "color": "orange"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "necco", "color": "yellow"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "necco", "color": "green"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "necco", "color": "purple"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "necco", "color": "pink"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "necco", "color": "white"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "necco", "color": "black"})
				})
			})
		})
		Convey("and a valid matrix using tag selectors", func() {
			m := matrix{
				Id: "candy",
				Spec: matrixDefinition{
					"color": []string{".hot_color", ".cool_color"}, // all but white and black
					"brand": []string{"*"},
				},
				Exclude: []matrixDefinition{
					{"brand": []string{".chewy"}, "color": []string{"brown", "blue"}},
					{"brand": []string{".chocolate"}, "color": []string{"purple"}},
					{"brand": []string{"!.chewy", "skittles"}, "color": []string{"pink"}},
					{"brand": []string{"!skittles !m&ms"}, "color": []string{"red", "blue"}},
				},
			}
			Convey("building a list of varations should succeed", func() {
				vs, errs := buildMatrixVariants(axes, ase, []matrix{m})
				So(errs, ShouldBeNil)
				Convey("and return the correct list of combinations", func() {
					// ensure all values are in there...
					So(len(vs), ShouldEqual, 16)
					vals := []matrixValue{}
					for _, d := range vs {
						vals = append(vals, d.matrixVal)
					}
					So(vals, ShouldContainResembling, matrixValue{"brand": "m&ms", "color": "red"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "m&ms", "color": "orange"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "m&ms", "color": "yellow"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "m&ms", "color": "green"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "m&ms", "color": "blue"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "m&ms", "color": "brown"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "skittles", "color": "red"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "skittles", "color": "orange"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "skittles", "color": "yellow"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "skittles", "color": "green"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "skittles", "color": "purple"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "necco", "color": "orange"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "necco", "color": "yellow"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "necco", "color": "green"})
					So(vals, ShouldContainResembling, matrixValue{"brand": "necco", "color": "purple"})
				})
			})
		})
		Convey("and a matrix that uses wrong axes", func() {
			m := matrix{
				Id: "candy",
				Spec: matrixDefinition{
					"strength": []string{"weak", "middle", "big-n-tough"},
				},
			}
			Convey("should fail to build", func() {
				vs, errs := buildMatrixVariants(axes, ase, []matrix{m})
				So(len(vs), ShouldEqual, 0)
				So(len(errs), ShouldEqual, 3)
			})
		})
		Convey("and a matrix that uses wrong axis values", func() {
			m := matrix{
				Id: "candy",
				Spec: matrixDefinition{
					"color": []string{"salmon", "infrared"},
				},
			}
			Convey("should fail to build", func() {
				vs, errs := buildMatrixVariants(axes, ase, []matrix{m})
				So(len(vs), ShouldEqual, 0)
				So(len(errs), ShouldEqual, 2)
			})
		})
	})
}

func TestMergeAxisValue(t *testing.T) {
	Convey("With a parserBV", t, func() {
		pbv := parserBV{
			RunOn:     []string{"basic_distro"},
			Modules:   []string{"basic_module"},
			Tags:      []string{"basic"},
			BatchTime: nil,
			Stepback:  nil,
			Expansions: map[string]string{
				"v1": "test",
			},
		}
		Convey("a valid axis value should merge successfully", func() {
			av := axisValue{
				RunOn:     []string{"special_distro"},
				Modules:   []string{"module++"},
				Tags:      []string{"enterprise"},
				BatchTime: new(int),
				Stepback:  new(bool),
				Variables: map[string]string{
					"v2": "new",
				},
			}
			So(pbv.mergeAxisValue(av), ShouldBeNil)
			So(pbv.RunOn, ShouldResemble, av.RunOn)
			So(pbv.Modules, ShouldResemble, av.Modules)
			So(pbv.Tags, ShouldContain, "basic")
			So(pbv.Tags, ShouldContain, "enterprise")
			So(pbv.Stepback, ShouldNotBeNil)
			So(pbv.BatchTime, ShouldNotBeNil)
			So(pbv.Expansions, ShouldResemble, command.Expansions{
				"v1": "test",
				"v2": "new",
			})
		})
		Convey("a valid axis value full of expansions should merge successfully", func() {
			av := axisValue{
				RunOn:   []string{"${v1}", "${v2}"},
				Modules: []string{"${v1}__"},
				Tags:    []string{"fat${v2}"},
				Variables: map[string]string{
					"v2": "${v1}!",
				},
			}
			So(pbv.mergeAxisValue(av), ShouldBeNil)
			So(pbv.RunOn, ShouldResemble, parserStringSlice{"test", "test!"})
			So(pbv.Modules, ShouldResemble, parserStringSlice{"test__"})
			So(pbv.Tags, ShouldContain, "basic")
			So(pbv.Tags, ShouldContain, "fattest!")
			So(pbv.Expansions, ShouldResemble, command.Expansions{
				"v1": "test",
				"v2": "test!",
			})
		})
		Convey("an axis value with a bad tag expansion should fail", func() {
			av := axisValue{
				Tags: []string{"fat${"},
			}
			So(pbv.mergeAxisValue(av), ShouldNotBeNil)
		})
		Convey("an axis value with a bad variables expansion should fail", func() {
			av := axisValue{
				Variables: map[string]string{
					"v2": "${sdsad",
				},
			}
			So(pbv.mergeAxisValue(av), ShouldNotBeNil)
		})
	})
}
