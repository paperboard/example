package main

import (
	"fmt"
	"image/color"
	"log"
	"runtime"
	"strings"

	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

const (
	windowWidth        = 600
	windowHeight       = 400
	bytesFloat32       = 4 // a float32 is 4 bytes
	bytesUint32        = 4 // a uint32 is 4 bytes
	vertexPositionSize = 3 // x,y,z
	vertexColorSize    = 4 // r,g,b,a
	verticesPerQuad    = 4 // a rectangle has 4 vertices
	indicesPerQuad     = 6 // a rectangle has 6 indices
)

var (
	program              uint32
	vbo                  uint32
	ibo                  uint32
	attribVertexPosition uint32
	attribVertexColor    uint32
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
	glfw.WindowHint(glfw.ContextVersionMajor, 2)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)

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

	// load game objects
	load()

	// pre-gameloop setup
	setup()

	// run gameloop
	for !window.ShouldClose() {

		// draw into buffer
		draw()

		// render buffer to screen
		window.SwapBuffers()

		// glfw events?
		glfw.PollEvents()

	}

}

func setup() {

	// cleared background color = gray
	gl.ClearColor(0.5, 0.5, 0.5, 1)

	// clear screen
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	// create shader program
	setupProgram()

	// prepare vbo/ibo buffers
	setupBuffers()

	// caculate camera matrices
	setupCamera()

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
//
var quadVertices = make([]float32, 0, 100) // size 100 doesn't matter
var quadColors = make([]uint32, 0, 100)
var quadIndices = make([]uint32, 0, 100)

func makeRectangle(w float32, h float32, z float32, c color.Color) {
	quadVertices = append(quadVertices, makeQuadVertices(w, h, z)...)
	quadColors = append(quadColors, makeQuadColors(c.RGBA())...)
	quadIndices = append(quadIndices, makeQuadIndices()...)
}

func makeQuadVertices(w, h, z float32) []float32 {
	return []float32{
		(w * 0.5), (h * 0.5), z, // v0 position = top-right
		-(w * 0.5), (h * 0.5), z, // v1 position = top-left
		-(w * 0.5), -(h * 0.5), z, // v2 position = bottom-left
		(w * 0.5), -(h * 0.5), z, // v3 position = bottom-right
	}
}

func makeQuadColors(r, g, b, a uint32) []uint32 {
	return []uint32{
		r, g, b, a,
		r, g, b, a,
		r, g, b, a,
		r, g, b, a,
	}
}

func makeQuadIndices() []uint32 {
	rectangleCount := len(quadVertices) / (verticesPerQuad * vertexPositionSize)
	i := uint32((rectangleCount - 1)) * verticesPerQuad
	return []uint32{
		i, i + 1, i + 2, // first triangle
		i, i + 2, i + 3, // second triangle
	}
}

func quadDebugPrint() {
	fmt.Printf("RECT_COUNT -- Rectangles: %v\n", len(quadIndices)/indicesPerQuad)
	fmt.Printf("RAW_LENGTH -- Rectangle has %v vertex\nVertices   %v (%v-per-vertex)\nColors     %v (%v-per-vertex)\nIndices    %v (%v-per-rectangle)\n", verticesPerQuad, len(quadVertices), vertexPositionSize, len(quadColors), vertexColorSize, len(quadIndices), indicesPerQuad)
}

func load() {

	// make red rectangle
	makeRectangle(2, 2, -1, color.NRGBA{1, 0, 0, 1})

	// make blue rectangle
	makeRectangle(1, 1, -1, color.NRGBA{0, 0, 1, 1})

	// print debug info for shapes
	quadDebugPrint()

}

func draw() {

	// clear screen
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	// bind program
	gl.UseProgram(program)

	// gl.Begin()
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)              // bind vertex buffer
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, ibo)      // bind indices buffer
	gl.EnableVertexAttribArray(attribVertexPosition) // enable vertex position
	gl.EnableVertexAttribArray(attribVertexColor)    // enable vertex color

	// configure and enable vertex position
	gl.VertexAttribPointer(attribVertexPosition, vertexPositionSize, gl.FLOAT, false, 0, gl.PtrOffset(0*bytesFloat32)) // PtrOffset = vertices position start at start of array (offset = 0)

	// configure and enable vertex color
	gl.VertexAttribPointer(attribVertexColor, vertexColorSize, gl.UNSIGNED_INT, false, 0, gl.PtrOffset(len(quadVertices)*bytesFloat32)) // PtrOffset = colors start after vertices position

	// draw rectangles
	gl.DrawElements(gl.TRIANGLES, int32(len(quadIndices)), gl.UNSIGNED_INT, gl.PtrOffset(0*bytesUint32))

	// gl.End()
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)                 // unbind vertex buffer
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, 0)         // unbind indices buffer
	gl.DisableVertexAttribArray(attribVertexPosition) // disable vertex position
	gl.DisableVertexAttribArray(attribVertexColor)    // disable vertex color

	// check for accumulated OpenGL errors
	checkGLError()

}

// https://en.wikipedia.org/wiki/Vertex_buffer_object
// https://www.songho.ca/opengl/gl_vbo.html#create
func setupBuffers() {

	// to be more efficient, vertices position are in float32 and color is in uint32
	bytesTotalSize := (len(quadVertices) * bytesFloat32) + (len(quadColors) * bytesUint32)

	// create VBOs
	gl.GenBuffers(1, &vbo) // for vertex buffer
	gl.GenBuffers(1, &ibo) // for index buffer

	// copy vertex data to VBO
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.BufferData(gl.ARRAY_BUFFER, bytesTotalSize, nil, gl.STATIC_DRAW)                                                // initalize but do not copy any data
	gl.BufferSubData(gl.ARRAY_BUFFER, 0*bytesFloat32, len(quadVertices)*bytesFloat32, gl.Ptr(quadVertices))            // copy vertices starting from 0 offest
	gl.BufferSubData(gl.ARRAY_BUFFER, len(quadVertices)*bytesFloat32, len(quadColors)*bytesUint32, gl.Ptr(quadColors)) // copy colors after vertices
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)

	// copy index data to VBO
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, ibo)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(quadIndices)*bytesUint32, gl.Ptr(quadIndices), gl.STATIC_DRAW)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, 0)

}

func setupProgram() {

	var err error

	// configure program, load shaders, and link attributes
	program, err = newProgram(vertexShader, fragmentShader)
	if err != nil {
		panic(err)
	}
	gl.UseProgram(program)

	// get attribute index for later use
	attribVertexPosition = uint32(gl.GetAttribLocation(program, gl.Str("vertexPosition\x00")))
	attribVertexColor = uint32(gl.GetAttribLocation(program, gl.Str("vertexColor\x00")))

}

// Object Space -> Eye/World Space -> Clip Space -> NDC Space -> Viewport/Window Space
//
// Transform 1: [ Object Coordinates ] transformed by [ ModelView ] matrix produces [ Eye/World Coordinates ]
// Transform 2: [ Eye/World Coordinates ] transformed by [ Projection ] matrix produces [ Clip Coordinates ]
// Transform 3: [ Clip Coordinates ] X, Y, Z divided by W produces [ Normalized Device Coordinates ] (aka. NDC)
// Transform 4: [ NDC ] scaled and translated by [ viewport ] parameters produces [ Window Coordinates ]
//
// The coordinate system on 3D space defined by the viewer.
// In eye coordinates in OpenGL, the viewer is located
// at the origin, looking in the direction of the negative
// z-axis, with the positive y-axis pointing upwards, and
// the positive x-axis pointing to the right. The modelview
// transformation maps objects into the eye coordinate system,
// and the projection transform maps eye coordinates to
// clip coordinates which is divided by "w" to produce
// normalized device coordinates ranging from (-1, 1) in
// all 3 axis (google "unit cube ndc"). Normally, it's
// convenient to set up the projection so one world coordinate
// unit (e.g. 1 meter) is equal to one screen pixel.
//
// Finally, by mapping NDC cube to window coordinates, screen
// graphics is produced. This final tranformation is a result
// of scaling and translating the NDC by viewport parameters
// given to gl.Viewport() and gl.DepthRange() functions.
//
// http://www.opengl-tutorial.org/beginners-tutorials/tutorial-3-matrices/#the-model-matrix
// https://www.opengl.org/archives/resources/faq/technical/transformations.htm
// http://math.hws.edu/graphicsbook/c3/s3.html (INTERACTIVE)
// https://stackoverflow.com/questions/15588860/what-exactly-are-eye-space-coordinates
// https://stackoverflow.com/questions/23309930/what-do-the-arguments-for-frustum-in-opengl-mean
// http://relativity.net.au/gaming/java/Frustum.html (INTERACTIVE)
// http://relativity.net.au/gaming/java/ProjectionMatrix.html
// https://www.sciencedirect.com/topics/computer-science/device-coordinate
// https://learnopengl.com/Getting-started/Coordinate-Systems
// https://learnopengl.com/Getting-started/Camera
// https://stackoverflow.com/questions/59262874/how-can-i-use-screen-space-coordinates-directly-with-opengl
// https://www.codeguru.com/cpp/misc/misc/graphics/article.php/c10123/Deriving-Projection-Matrices.htm#page-2
func setupCamera() {

	// do not render parts of shapes (pixels) that will
	// anyhow be covered up by higher z-axis shapes (pixels)
	// so that we are drawing pixels more efficiently
	gl.Enable(gl.DEPTH_TEST)

	// if multiple shapes have same z-value, take their
	// draw order in account and show if possible
	gl.DepthFunc(gl.LEQUAL)

	// CREATE (PRESPECTIVE) PROJECTION MATRIX
	// a matrix to transform from eye to NDC coordinates
	projection := mgl32.Perspective(mgl32.DegToRad(90), float32(windowWidth)/windowHeight, 1, 100)
	projectionUniform := gl.GetUniformLocation(program, gl.Str("projection\x00"))
	gl.UniformMatrix4fv(projectionUniform, 1, false, &projection[0])

	// CREATE (CAMERA) VIEW MATRIX
	// a matrix to transform from eye to NDC coordinates
	// 1st arg = camera position
	// 2nd arg = camera directional vector
	// 3rd arg = up is Y axis
	camera := mgl32.LookAtV(mgl32.Vec3{0, 0, 0}, mgl32.Vec3{0, 0, -1}, mgl32.Vec3{0, 1, 0})
	cameraUniform := gl.GetUniformLocation(program, gl.Str("camera\x00"))
	gl.UniformMatrix4fv(cameraUniform, 1, false, &camera[0])

	// CREATE (OBJECT) MODEL MATRIX
	// a matrix to transform from object to eye coordinates
	model := mgl32.Ident4()
	modelUniform := gl.GetUniformLocation(program, gl.Str("model\x00"))
	gl.UniformMatrix4fv(modelUniform, 1, false, &model[0])

}

var vertexShader = `
#version 120

// input
uniform mat4 projection;
uniform mat4 camera;
uniform mat4 model;

// input
attribute vec3 vertexPosition;
attribute vec4 vertexColor;

// output
varying vec4 fragmentColor;

void main() {
	fragmentColor = vertexColor;
	gl_Position = projection * camera * model * vec4(vertexPosition, 1);
}
` + "\x00"

var fragmentShader = `
#version 120

// input
varying vec4 fragmentColor;

void main() {
	gl_FragColor = fragmentColor;
}
` + "\x00"

func newProgram(vertexShaderSource, fragmentShaderSource string) (uint32, error) {

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

var GL_ERROR_LOOKUP = map[uint32]string{
	0x500: `GL_INVALID_ENUM`,
	0x501: `GL_INVALID_VALUE`,
	0x502: `GL_INVALID_OPERATION`,
	0x503: `GL_STACK_OVERFLOW`,
	0x504: `GL_STACK_UNDERFLOW`,
	0x505: `GL_OUT_OF_MEMORY`,
	0x506: `GL_INVALID_FRAMEBUFFER_OPERATION`,
	0x507: `GL_CONTEXT_LOST`,
}

func panic_GL_ERROR(errcode uint32) {
	if errstr, ok := GL_ERROR_LOOKUP[errcode]; ok {
		panic(fmt.Sprintf("GL_ERROR: %s\n", errstr))
	} else {
		panic(fmt.Sprintf("GL_ERROR UNKNOWN: %v\n", errcode))
	}
}

func checkGLError() {
	for {
		glerr := gl.GetError()
		if glerr == gl.NO_ERROR {
			break
		}
		panic_GL_ERROR(glerr)
	}
}
