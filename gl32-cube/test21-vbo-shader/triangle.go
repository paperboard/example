package main

import (
	"fmt"
	"log"
	"runtime"

	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

const (
	windowWidth        = 600
	windowHeight       = 400
	floatSizeInBytes   = 4 // a float32 is 4 bytes
	vertexPositionSize = 3 // x,y,z
	vertexColorSize    = 3 // r,g,b
	vertexSize         = 6 // vertexPositionSize + vertexColorSize
)

var (
	vbo uint32
	ibo uint32
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
	gl.Clear(gl.COLOR_BUFFER_BIT)

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
// vertex ( position + color ) array
// vertexs needed for 2 triangles that cover a rectangular screen
var quadVertices = []float32{
	(windowWidth * 0.5), (windowHeight * 0.5), -1, // v0 position = top-right
	0, 0, 0, // v0 color = black
	-(windowWidth * 0.5), (windowHeight * 0.5), -1, // v1 position = top-left
	1, 0, 0, // v1 color = red
	-(windowWidth * 0.5), -(windowHeight * 0.5), -1, // v2 position = bottom-left
	0, 0, 0, // v2 color = black
	(windowWidth * 0.5), -(windowHeight * 0.5), -1, // v3 position = bottom-right
	0, 0, 1, // v3 color = blue
}

// indices array
var quadIndices = []int32{
	0, 1, 2, // first triangle
	0, 2, 3, // second triangle
}

func draw() {

	// gl.Begin()
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)         // bind vertex buffer
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, ibo) // bind indices buffer
	gl.EnableClientState(gl.VERTEX_ARRAY)       // enable vertex position
	gl.EnableClientState(gl.COLOR_ARRAY)        // enable vertex color

	// configure and enable vertex position
	gl.VertexPointer(vertexPositionSize, gl.FLOAT, vertexSize*floatSizeInBytes, gl.PtrOffset(0*floatSizeInBytes))

	// configure and enable vertex color
	gl.ColorPointer(vertexColorSize, gl.FLOAT, vertexSize*floatSizeInBytes, gl.PtrOffset(vertexPositionSize*floatSizeInBytes))

	// draw triangles
	gl.DrawElements(gl.TRIANGLES, int32(len(quadIndices)), gl.UNSIGNED_INT, gl.PtrOffset(0*floatSizeInBytes))

	// gl.End()
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)         // unbind vertex buffer
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, 0) // unbind indices buffer
	gl.DisableClientState(gl.VERTEX_ARRAY)    // disable vertex position
	gl.DisableClientState(gl.COLOR_ARRAY)     // disable vertex color

	// check for accumulated OpenGL errors
	for {
		glerr := gl.GetError()
		if glerr == gl.NO_ERROR {
			break
		}
		print_GL_ERROR(glerr)
	}

}

// https://en.wikipedia.org/wiki/Vertex_buffer_object
// https://www.songho.ca/opengl/gl_vbo.html#create
func setupBuffers() {

	// create VBOs
	gl.GenBuffers(1, &vbo) // for vertex buffer
	gl.GenBuffers(1, &ibo) // for index buffer

	// copy vertex data to VBO
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(quadVertices)*floatSizeInBytes, gl.Ptr(quadVertices), gl.STATIC_DRAW)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)

	// copy index data to VBO
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, ibo)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(quadIndices)*floatSizeInBytes, gl.Ptr(quadIndices), gl.STATIC_DRAW)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, 0)

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
// https://www.opengl.org/archives/resources/faq/technical/transformations.htm
// http://math.hws.edu/graphicsbook/c3/s3.html (INTERACTIVE)
// https://stackoverflow.com/questions/15588860/what-exactly-are-eye-space-coordinates
// https://stackoverflow.com/questions/23309930/what-do-the-arguments-for-frustum-in-opengl-mean
// http://relativity.net.au/gaming/java/Frustum.html (INTERACTIVE)
// http://relativity.net.au/gaming/java/ProjectionMatrix.html
// https://www.sciencedirect.com/topics/computer-science/device-coordinate
// https://learnopengl.com/Getting-started/Coordinate-Systems
func setupCamera() {

	// from the viewpoint of the camera at centerpoint (0,0,0)
	frustumLeft := -windowWidth * 0.5
	frustumRight := windowWidth * 0.5
	frustumBottom := -windowHeight * 0.5
	frustumTop := windowHeight * 0.5

	// CREATE (PRESPECTIVE) PROJECTION MATRIX
	// a matrix to transform from eye to NDC coordinates
	gl.MatrixMode(gl.PROJECTION)                                             // bind to projection matrix
	gl.LoadIdentity()                                                        // clear matrix by replacing with identity matrix
	gl.Frustum(frustumLeft, frustumRight, frustumBottom, frustumTop, 1, 100) // produce projection matrix && dot product it with identity matrix

	// unbind from projection matrix (as we are done)
	// and bind to modelview matrix and clear it.
	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()
	// TODO: depending on how we decide to use Object coordinate space,
	//       we would need to set a modelview matrix to tranform from
	//       object to eye coordinates.

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

func print_GL_ERROR(errcode uint32) {
	if errstr, ok := GL_ERROR_LOOKUP[errcode]; ok {
		fmt.Printf("GL_ERROR: %s\n", errstr)
	} else {
		fmt.Printf("GL_ERROR UNKNOWN: %v\n", errcode)
	}
}
