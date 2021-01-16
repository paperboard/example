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
	bytesUint16        = 2 // a uint16 is 2 bytes
	bytesUint8         = 1 // a uint8 has 1 byte
	vertexPositionSize = 3 // x,y,z = points in 3D space
	vertexTexCoordSize = 2 // x,y = texture coordinates
	vertexColorSize    = 4 // r,g,b,a = color w/ transparency
	verticesPerQuad    = 4 // a rectangle has 4 vertices
	indicesPerQuad     = 6 // a rectangle has 6 indices
)

var (
	quadVertices    = make([]float32, 0, 100)
	quadTexCoords   = make([]uint8, 0, 100)
	quadColors      = make([]uint32, 0, 100)
	quadIndices     = make([]uint16, 0, 100)
	offsetVertices  = 0
	offsetTexCoords = 0
	offsetColors    = 0
	vboBytesTotal   = 0 // total bytes needed for VBO buffer (quadVertices + quadTexCoords + quadColors)
)

var (
	programScreen           uint32 // connects vertex and fragment shaders (Screen shaders)
	programFramebuffer      uint32 // connects vertex and fragment shaders (Framebuffer shaders)
	fbo                     uint32 // off-screen rendering using framebuffer
	fboTexture              uint32 // texture attachment for framebuffer color component (to act as proxy for default framebuffer aka. screen)
	fboRenderbuffer         uint32 // renderbuffer attachment for framebuffer depth & stencil components (to act as proxy for default framebuffer aka. screen)
	vbo                     uint32 // stores vertex position, color, texture, and normal array data
	ibo                     uint32 // stores sets of indicies to draw that make up elements (e.g. triangles)
	attribVertexPosition    uint32 // reference to position input for shader variable (Framebuffer shaders)
	attribVertexTexCoord    uint32 // reference to texture coordinate input for shader variable (Framebuffer shaders)
	attribVertexColor       uint32 // reference to color input for shader variable (Framebuffer shaders)
	attribVertexPositionFBO uint32 // reference to position input for shader variable (Screen shaders)
	attribVertexTextureFBO  uint32 // reference to texture (replacement for Color) input for shader variable (Screen shaders)
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

	// one-time clear screen to yellow
	gl.ClearColor(1, 1, 0, 1)
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	// do not render parts of shapes (pixels) that will
	// anyhow be covered up by higher z-axis shapes (pixels)
	// so that we are drawing pixels more efficiently
	gl.Enable(gl.DEPTH_TEST)

	// if multiple shapes have same z-value, take their
	// draw order in account and show if possible
	gl.DepthFunc(gl.LEQUAL)

	// enable textures
	gl.Enable(gl.TEXTURE_2D)

	// create shader programs
	setupProgram_Screen()
	setupProgram_Framebuffer()

	// prepare vbo/ibo buffers
	setupBuffers()

	// caculate camera matrices
	setupCamera(90, mgl32.Vec3{2, 2, 2}, mgl32.Vec3{0, 0, -1})

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
func makeQuadVertices(w, h, z float32) []float32 {
	return []float32{
		(w * 0.5), (h * 0.5), z, // v0 position = top-right
		-(w * 0.5), (h * 0.5), z, // v1 position = top-left
		-(w * 0.5), -(h * 0.5), z, // v2 position = bottom-left
		(w * 0.5), -(h * 0.5), z, // v3 position = bottom-right
	}
}

// texture 2D unit quad
//
// (0,1)    (1,1)
//  v1------v0
//  |       |
//  |       |
//  |       |
//  v2------v3
// (0,0)    (1,0)
//
// https://web.cse.ohio-state.edu/~shen.94/581/Site/Slides_files/texture.pdf
func makeQuadTextureCoord() []uint8 {
	return []uint8{
		1, 1, // v0 = texel @ top-right in texture coordinate system
		0, 1, // v1 = texel @ top-left in texture coordinate system
		0, 0, // v2 = texel @ bottom-left in texture coordinate system
		1, 0, // v3 = texel @ bottom-right in texture coordinate system
	}
}

func makeQuadColors(r, g, b, a uint32) []uint32 {
	// all 4 vertex (v0, v1, v2, v3) should have same color
	return []uint32{
		r, g, b, a, // v0
		r, g, b, a, // v1
		r, g, b, a, // v2
		r, g, b, a, // v3
	}
}

func makeQuadIndices() []uint16 {
	rectangleCount := len(quadVertices) / (verticesPerQuad * vertexPositionSize)
	i := uint16((rectangleCount - 1)) * verticesPerQuad
	return []uint16{
		i, i + 1, i + 2, // first triangle
		i, i + 2, i + 3, // second triangle
	}
}

func quadDebugPrint() {
	fmt.Printf("RECT_COUNT -- Rectangles: %v\n", len(quadIndices)/indicesPerQuad)
	fmt.Printf("RAW_LENGTH -- Rectangle has %v vertex\nVertices   %v (%v-per-vertex)\nTexCoord   %v (%v-per-vertex)\nColors     %v (%v-per-vertex)\nIndices    %v (%v-per-rectangle)\n", verticesPerQuad, len(quadVertices), vertexPositionSize, len(quadTexCoords), vertexTexCoordSize, len(quadColors), vertexColorSize, len(quadIndices), indicesPerQuad)
}

func drawRectangle(w float32, h float32, z float32, c color.Color) {
	quadVertices = append(quadVertices, makeQuadVertices(w, h, z)...)
	quadTexCoords = append(quadTexCoords, makeQuadTextureCoord()...)
	quadColors = append(quadColors, makeQuadColors(c.RGBA())...)
	quadIndices = append(quadIndices, makeQuadIndices()...)
}

func load() {

	// draw red rectangle
	drawRectangle(2, 2, -1.2, color.NRGBA{1, 0, 0, 1})

	// draw blue rectangle
	drawRectangle(1, 1, -1.1, color.NRGBA{0, 0, 1, 1})

	// print debug info for shapes
	quadDebugPrint()

}

func draw() {

	// bind offscreen framebuffer
	bindProxyScreen()

	// gl.Begin()
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)              // bind vertex buffer
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, ibo)      // bind indices buffer
	gl.EnableVertexAttribArray(attribVertexPosition) // enable vertex position
	gl.EnableVertexAttribArray(attribVertexTexCoord) // enable vertex texture coordinate
	gl.EnableVertexAttribArray(attribVertexColor)    // enable vertex color

	// configure and enable vertex position
	gl.VertexAttribPointer(attribVertexPosition, vertexPositionSize, gl.FLOAT, false, 0, gl.PtrOffset(offsetVertices))

	// configure and enable vertex texture coordinate
	gl.VertexAttribPointer(attribVertexTexCoord, vertexTexCoordSize, gl.UNSIGNED_BYTE, false, 0, gl.PtrOffset(offsetTexCoords))

	// configure and enable vertex color
	gl.VertexAttribPointer(attribVertexColor, vertexColorSize, gl.UNSIGNED_INT, false, 0, gl.PtrOffset(offsetColors))

	// draw rectangles
	gl.DrawElements(gl.TRIANGLES, int32(len(quadIndices)), gl.UNSIGNED_SHORT, gl.PtrOffset(0*bytesUint16))

	// gl.End()
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)                 // unbind vertex buffer
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, 0)         // unbind indices buffer
	gl.DisableVertexAttribArray(attribVertexPosition) // disable vertex position
	gl.DisableVertexAttribArray(attribVertexTexCoord) // disable vertex texture coordinate
	gl.DisableVertexAttribArray(attribVertexColor)    // disable vertex color

	// unbind proxy screen
	unbindProxyScreen()

	// using the proxy screen's rendered image, overlay on real screen using a single quad
	renderProxyToScreen()

	// check for accumulated OpenGL errors
	checkGLError()

}

// use proxy offscreen rendering using framebuffers
func bindProxyScreen() {

	// bind Framebuffer program
	gl.UseProgram(programFramebuffer)

	// bind proxy framebuffer instead of default framebuffer
	gl.BindFramebufferEXT(gl.FRAMEBUFFER_EXT, fbo)

	// clear proxy screen to gray
	gl.ClearColor(0.5, 0.5, 0.5, 1)
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	// ensure depth test is enabled during proxy screen usage
	gl.Enable(gl.DEPTH_TEST)

}

// use default (real) screen for rendering by unbinding proxy screen
func unbindProxyScreen() {

	// bind Screen program
	gl.UseProgram(programScreen)

	// unbind proxy framebuffer and set back to default framebuffer
	gl.BindFramebufferEXT(gl.FRAMEBUFFER_EXT, 0)

	// clear screen to black
	gl.ClearColor(0, 0, 0, 1)
	gl.Clear(gl.COLOR_BUFFER_BIT) // no need to clear depth, we will disable depth

	// disable depth test
	gl.Disable(gl.DEPTH_TEST)

}

func renderProxyToScreen() {

}

// https://en.wikipedia.org/wiki/Vertex_buffer_object
// https://www.songho.ca/opengl/gl_vbo.html#create
// https://learnopengl.com/Advanced-OpenGL/Framebuffers
func setupBuffers() {

	// use PROXY program
	gl.UseProgram(programFramebuffer)

	// to be more efficient, vertices position are in float32, texture coordinate in uint8, and color is in uint32
	vboBytesTotal = (len(quadVertices) * bytesFloat32) + (len(quadTexCoords) * bytesUint8) + (len(quadColors) * bytesUint32)

	// data offsets
	offsetVertices = 0 * bytesFloat32
	offsetTexCoords = offsetVertices + len(quadVertices)*bytesFloat32
	offsetColors = offsetTexCoords + len(quadTexCoords)*bytesUint8

	// create FBO and bind to it
	gl.GenFramebuffersEXT(1, &fbo) // offscreen rendering use framebuffer extension
	gl.BindFramebufferEXT(gl.FRAMEBUFFER_EXT, fbo)

	// attach texture to FBO (color buffer component)
	attachTexture()

	/// attach renderbuffer to FBO (combined depth and stencil buffer component)
	attachRenderbuffer()

	// check if FBO is ready and valid
	if gl.CheckFramebufferStatusEXT(gl.FRAMEBUFFER_EXT) != gl.FRAMEBUFFER_COMPLETE_EXT {
		panic("Framebuffer (FBO) FATAL ERROR")
	}

	// create VBOs
	gl.GenBuffers(1, &vbo) // buffer for vertex position, texture coordinate, and color
	gl.GenBuffers(1, &ibo) // buffer for vertex indices

	// copy vertex data to VBO
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.BufferData(gl.ARRAY_BUFFER, vboBytesTotal, nil, gl.STATIC_DRAW)                                       // initalize but do not copy any data
	gl.BufferSubData(gl.ARRAY_BUFFER, offsetVertices, len(quadVertices)*bytesFloat32, gl.Ptr(quadVertices))  // copy vertices starting from 0 offest
	gl.BufferSubData(gl.ARRAY_BUFFER, offsetTexCoords, len(quadTexCoords)*bytesUint8, gl.Ptr(quadTexCoords)) // copy textures after vertices
	gl.BufferSubData(gl.ARRAY_BUFFER, offsetColors, len(quadColors)*bytesUint32, gl.Ptr(quadColors))         // copy colors after textures
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)

	// copy index data to VBO
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, ibo)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(quadIndices)*bytesUint16, gl.Ptr(quadIndices), gl.STATIC_DRAW)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, 0)

	// unbind FBO
	gl.BindFramebufferEXT(gl.FRAMEBUFFER_EXT, 0)

	// unbind PROXY program
	gl.UseProgram(0)

}

// should only be called by setupBuffers()
func attachTexture() {

	// create texture for framebuffer attachment, and bind to it
	gl.GenTextures(1, &fboTexture)
	gl.BindTexture(gl.TEXTURE_2D, fboTexture)

	// initalize texture (memory space and min/mag filters)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGB, windowWidth, windowHeight, 0, gl.RGB, gl.UNSIGNED_BYTE, nil)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)

	// unbind texture
	gl.BindTexture(gl.TEXTURE_2D, 0)

	// attach texture to framebuffer
	gl.FramebufferTexture2DEXT(gl.FRAMEBUFFER_EXT, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, fboTexture, 0)

}

// should only be called by setupBuffers()
func attachRenderbuffer() {

	// create renderbuffer for depth and stencil testing. and bind to it
	gl.GenRenderbuffersEXT(1, &fboRenderbuffer)
	gl.BindRenderbufferEXT(gl.RENDERBUFFER_EXT, fboRenderbuffer)

	// initalize renderbuffer memory space
	gl.RenderbufferStorageEXT(gl.RENDERBUFFER_EXT, gl.DEPTH24_STENCIL8, windowWidth, windowHeight)

	// unbind renderbuffer
	gl.BindRenderbufferEXT(gl.RENDERBUFFER_EXT, 0)

	// attach renderbuffer to framebuffer
	gl.FramebufferRenderbufferEXT(gl.FRAMEBUFFER_EXT, gl.DEPTH_STENCIL_ATTACHMENT, gl.RENDERBUFFER_EXT, fboRenderbuffer)

}

func setupProgram_Screen() {

	var err error

	// configure program, load shaders, and link attributes
	programScreen, err = newProgram(vertexShaderScreen, fragmentShaderScreen)
	if err != nil {
		panic(err)
	}
	gl.UseProgram(programScreen)

	// get attribute index for later use
	attribVertexPositionFBO = uint32(gl.GetAttribLocation(programScreen, gl.Str("vertexPositionFBO\x00")))
	attribVertexTextureFBO = uint32(gl.GetAttribLocation(programScreen, gl.Str("vertexTextureFBO\x00")))

	// unbind program
	gl.UseProgram(0)

}

func setupProgram_Framebuffer() {

	var err error

	// configure program, load shaders, and link attributes
	programFramebuffer, err = newProgram(vertexShaderFramebuffer, fragmentShaderFramebuffer)
	if err != nil {
		panic(err)
	}
	gl.UseProgram(programFramebuffer)

	// get attribute index for later use
	attribVertexPosition = uint32(gl.GetAttribLocation(programFramebuffer, gl.Str("vertexPosition\x00")))
	attribVertexTexCoord = uint32(gl.GetAttribLocation(programFramebuffer, gl.Str("vertexTexCoord\x00")))
	attribVertexColor = uint32(gl.GetAttribLocation(programFramebuffer, gl.Str("vertexColor\x00")))

	fmt.Printf("attribVertexPosition: %v attribVertexTexCoord: %v attribVertexColor: %v\n", attribVertexPosition, attribVertexTexCoord, attribVertexColor)

	// unbind program
	gl.UseProgram(0)

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
func setupCamera(fov float32, cameraposition mgl32.Vec3, target mgl32.Vec3) {

	// use PROXY program
	gl.UseProgram(programFramebuffer)

	// CREATE (PRESPECTIVE) PROJECTION MATRIX
	// a matrix to transform from eye to NDC coordinates
	projection := mgl32.Perspective(mgl32.DegToRad(fov), float32(windowWidth)/windowHeight, 0.1, 10.0)
	projectionUniform := gl.GetUniformLocation(programFramebuffer, gl.Str("projection\x00"))
	gl.UniformMatrix4fv(projectionUniform, 1, false, &projection[0])

	// CREATE (CAMERA) VIEW MATRIX
	// a matrix to transform from eye to NDC coordinates
	camera := mgl32.LookAtV(cameraposition, target, mgl32.Vec3{0, 1, 0})
	cameraUniform := gl.GetUniformLocation(programFramebuffer, gl.Str("camera\x00"))
	gl.UniformMatrix4fv(cameraUniform, 1, false, &camera[0])

	// CREATE (OBJECT) MODEL MATRIX
	// a matrix to transform from object to eye coordinates
	model := mgl32.Ident4()
	modelUniform := gl.GetUniformLocation(programFramebuffer, gl.Str("model\x00"))
	gl.UniformMatrix4fv(modelUniform, 1, false, &model[0])

	// unbind PROXY program
	gl.UseProgram(0)

}

var vertexShaderFramebuffer = `
#version 120

// input
uniform mat4 projection;
uniform mat4 camera;
uniform mat4 model;

// input
attribute vec3 vertexPosition;
attribute vec2 vertexTexCoord;
attribute vec4 vertexColor;

// output
varying vec2 fragmentTexCoord;
varying vec4 fragmentColor;

void main() {
	fragmentTexCoord = vertexTexCoord;
	fragmentColor = vertexColor;
	gl_Position = projection * camera * model * vec4(vertexPosition, 1);
}
` + "\x00"

var fragmentShaderFramebuffer = `
#version 120

// input
varying vec2 fragmentTexCoord;
varying vec4 fragmentColor;

void main() {
	//vec3 fragColor = fragmentColor;
	//fragColor *= texture2D(map0, fragmentTexCoord).rgb; 
	//gl_FragColor = vec4(fragColor, fragmentColor.a);
	gl_FragColor = fragmentColor;
}
` + "\x00"

var vertexShaderScreen = `
#version 120

// input
attribute vec2 vertexPositionFBO;
attribute vec2 vertexTextureFBO;

// output
varying vec2 fragmentTextureFBO;

void main() {
	fragmentTextureFBO = vertexTextureFBO;
	gl_Position = vec4(vertexPositionFBO, 0, 1);
}
` + "\x00"

var fragmentShaderScreen = `
#version 120

// input
uniform sampler2D screenTexture;

// input
varying vec2 fragmentTextureFBO;

void main() {
	gl_FragColor = texture2D(screenTexture, fragmentTextureFBO);
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
