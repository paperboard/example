package main

import (
	"fmt"
	"log"
	"runtime"
	"strings"

	"github.com/go-gl/gl/v3.2-compatibility/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

var (
	program      uint32
	vbo          uint32
	ibo          uint32
	attribVertex uint32
	attribColor  uint32
)

const (
	attribVertexOffset = 0 // vertex co-ordinates begins at the start of array
	attribColorOffset  = 3 // color data begins after vertex co-ordinates
	attribVertexCount  = 3 // x,y,z
	attribColorCount   = 3 // r,g,b
	vertexSize         = 6 // attribVertexCount + attribColorCount
	floatSizeInBytes   = 4 // float is 4 bytes
	windowWidth        = 600
	windowHeight       = 400
)

func init() {
	// glfw must be on main thread
	runtime.LockOSThread()
}

func main() {

	// initalize glfw
	err := glfw.Init()
	if err != nil {
		log.Fatalln("failed to initialize glfw:", err)
	}
	defer glfw.Terminate()

	// use OpenGL v2.1
	glfw.WindowHint(glfw.Resizable, glfw.False)
	glfw.WindowHint(glfw.ContextVersionMajor, 3)
	glfw.WindowHint(glfw.ContextVersionMinor, 2)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)

	// create window handle
	window, err := glfw.CreateWindow(windowWidth, windowHeight, "Triangle 3D", nil, nil)
	if err != nil {
		panic(err)
	}
	window.MakeContextCurrent()

	// initialize OpenGL
	err = gl.Init()
	if err != nil {
		panic(err)
	}
	fmt.Println("OpenGL version", gl.GoStr(gl.GetString(gl.VERSION)))

	// cleared background color = gray
	gl.ClearColor(0.5, 0.5, 0.5, 1.0)

	// enable 3D features
	gl.Enable(gl.DEPTH_TEST)
	gl.FrontFace(gl.CCW)
	gl.DepthFunc(gl.LEQUAL)

	// make program with shaders and vbo/ibo
	setupScene()

	// game loop
	for !window.ShouldClose() {

		// draw into buffer
		drawScene()

		// render buffer to screen
		window.SwapBuffers()

		// glfw events?
		glfw.PollEvents()

	}

}

// https://www.songho.ca/opengl/gl_vbo.html#create
func setupScene() {

	var err error

	// configure the vertex and fragment shaders
	program, err = newProgram(vertexShader, fragmentShader, []string{"vert", "vertColor"})
	if err != nil {
		panic(err)
	}
	gl.UseProgram(program)

	// create VBOs
	gl.GenBuffers(1, &vbo) // for vertex buffer
	gl.GenBuffers(1, &ibo) // for index buffer

	// copy vertex data to VBO
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(cubeVertices)*floatSizeInBytes, gl.Ptr(cubeVertices), gl.STATIC_DRAW)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)

	// copy index data to VBO
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, ibo)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(cubeIndices)*floatSizeInBytes, gl.Ptr(cubeIndices), gl.STATIC_DRAW)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, 0)

}

func drawScene() {

	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	// load program with shaders
	gl.UseProgram(program)

	// camera projection
	setupCamera()

	// activate attribs before drawing
	attribVertex = uint32(gl.GetAttribLocation(program, gl.Str("vert\x00")))
	attribColor = uint32(gl.GetAttribLocation(program, gl.Str("vertColor\x00")))
	gl.EnableVertexAttribArray(attribVertex)
	gl.EnableVertexAttribArray(attribColor)

	// bind vertex buffer
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)

	// set vertex array
	gl.VertexAttribPointer(attribVertex, attribVertexCount, gl.FLOAT, false, vertexSize*floatSizeInBytes, gl.PtrOffset(attribVertexOffset*floatSizeInBytes)) // PtrOffset = 0
	gl.VertexAttribPointer(attribColor, attribColorCount, gl.FLOAT, false, vertexSize*floatSizeInBytes, gl.PtrOffset(attribColorOffset*floatSizeInBytes))    // PtrOffset = 12

	// bind indices buffer
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, ibo)

	// set indices array
	gl.DrawElements(gl.TRIANGLES, 3, gl.UNSIGNED_INT, gl.PtrOffset(0))

	// deactivate attributes after drawing
	gl.DisableVertexAttribArray(attribVertex) // deactivate vertex position
	gl.DisableVertexAttribArray(attribColor)  // deactivate color data

	// bind with 0, so, switch back to normal pointer operation
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, 0)

}

func setupCamera() {

	// generate perspective matrix
	projection := mgl32.Perspective(mgl32.DegToRad(45.0), float32(windowWidth)/windowHeight, 0.1, 10.0)
	projectionUniform := gl.GetUniformLocation(program, gl.Str("projection\x00"))
	gl.UniformMatrix4fv(projectionUniform, 1, false, &projection[0])

	// from world space to eye space
	camera := mgl32.LookAtV(mgl32.Vec3{3, 3, 3}, mgl32.Vec3{0, 0, 0}, mgl32.Vec3{0, 1, 0})
	cameraUniform := gl.GetUniformLocation(program, gl.Str("camera\x00"))
	gl.UniformMatrix4fv(cameraUniform, 1, false, &camera[0])

	// model view
	model := mgl32.Ident4()
	modelUniform := gl.GetUniformLocation(program, gl.Str("model\x00"))
	gl.UniformMatrix4fv(modelUniform, 1, false, &model[0])

	// bind output color
	gl.BindFragDataLocation(program, 0, gl.Str("outputColor\x00"))

}

// unit cube
//
//    v6----- v5
//   /|      /|
//  v1------v0|
//  | |     | |
//  | v7----|-v4
//  |/      |/
//  v2------v3

// vertex position array // TRIANGLE
var cubeVertices = []float32{
	0.5, 0.5, -1, 1, 0, 0, // v0
	-.5, 0.5, -1, 1, 0, 0, // v1
	-.5, -.5, -1, 1, 0, 0, // v2
}
var cubeIndices = []float32{0, 1, 2}

var vertexShader = `
#version 330

uniform mat4 projection;  //in
uniform mat4 camera;      //in
uniform mat4 model;       //in

in vec3 vert;      //in
in vec3 vertColor; //in

out vec3 fragColor;   //out

void main() {
	fragColor = vertColor;
	gl_Position = projection * camera * model * vec4(vert, 1);
}
` + "\x00"

var fragmentShader = `
#version 330

in vec3 fragColor;   //in
out vec4 outputColor; //out

void main() {
	outputColor = vec4(fragColor, 1);
}
` + "\x00"

func newProgram(vertexShaderSource, fragmentShaderSource string, attributes []string) (uint32, error) {

	vertexShader, err := compileShader(vertexShaderSource, gl.VERTEX_SHADER)
	if err != nil {
		return 0, err
	}

	fragmentShader, err := compileShader(fragmentShaderSource, gl.FRAGMENT_SHADER)
	if err != nil {
		return 0, err
	}

	program := gl.CreateProgram()

	gl.AttachShader(program, vertexShader)
	gl.AttachShader(program, fragmentShader)

	/*
		for i, name := range attributes {
			l, free := gl.Strs(name + "\x00")
			gl.BindAttribLocation(program, uint32(i), *l)
			free()
		}
	*/

	gl.LinkProgram(program)

	var status int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {

		var logLength int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)

		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetProgramInfoLog(program, logLength, nil, gl.Str(log))

		return 0, fmt.Errorf("failed to link program: %v", log)

	}

	gl.DeleteShader(vertexShader)
	gl.DeleteShader(fragmentShader)

	return program, nil

}

func compileShader(source string, shaderType uint32) (uint32, error) {

	shader := gl.CreateShader(shaderType)

	csources, free := gl.Strs(source)
	gl.ShaderSource(shader, 1, csources, nil)
	free()
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {

		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)

		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetShaderInfoLog(shader, logLength, nil, gl.Str(log))

		return 0, fmt.Errorf("failed to compile %v: %v", source, log)

	}

	return shader, nil

}
