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
	windowWidth        = 600 // intended game screen width, but will become larger on high-dpi screens
	windowHeight       = 400 // intended game screen height, but will become larger on high-dpi screens
	bytesFloat32       = 4   // a float32 is 4 bytes
	bytesUint32        = 4   // a uint32 is 4 bytes
	bytesUint16        = 2   // a uint16 is 2 bytes
	bytesUint8         = 1   // a uint8 has 1 byte
	vertexPositionSize = 3   // x,y,z = points in 3D space
	vertexTexCoordSize = 2   // x,y = texture coordinates
	vertexColorSize    = 4   // r,g,b,a = color w/ transparency
	verticesPerQuad    = 4   // a rectangle has 4 vertices
	indicesPerQuad     = 6   // a rectangle has 6 indices
)

var (
	dpiScaleX float32 // to adjust width for high dpi/resolution monitors
	dpiScaleY float32 // to adjust height for high dpi/resolution monitors
)

var (
	ctxScreen      = &ContextScreen{}
	ctxFramebuffer = &ContextFramebuffer{}
)

// ContextScreen is a real screen
type ContextScreen struct {
	quads                *ElementQuads
	program              uint32 // connects vertex and fragment shaders (Screen shaders)
	vbo                  uint32 // stores vertex position, color, texture, and normal array data
	ibo                  uint32 // stores sets of indicies to draw that make up elements (e.g. triangles)
	attribVertexPosition uint32 // reference to position input for shader variable (Screen shaders)
	attribVertexTexCoord uint32 // reference to texture coordinate input for shader variable (Screen shaders)
}

// ContextFramebuffer is a proxy screen
type ContextFramebuffer struct {
	quads                *ElementQuads
	program              uint32 // connects vertex and fragment shaders (Framebuffer shaders)
	fbo                  uint32 // off-screen rendering using framebuffer
	fboTexture           uint32 // texture attachment for framebuffer color component (to act as proxy for default framebuffer aka. screen)
	fboRenderbuffer      uint32 // renderbuffer attachment for framebuffer depth & stencil components (to act as proxy for default framebuffer aka. screen)
	vbo                  uint32 // stores vertex position, color, texture, and normal array data
	ibo                  uint32 // stores sets of indicies to draw that make up elements (e.g. triangles)
	attribVertexPosition uint32 // reference to position input for shader variable (Framebuffer shaders)
	attribVertexTexCoord uint32 // reference to texture coordinate input for shader variable (Framebuffer shaders)
	attribVertexColor    uint32 // reference to color input for shader variable (Framebuffer shaders)
}

// ElementQuads hold draw elements used by both "real screen" (ContextScreen) and "proxy screen" (ContextFramebuffer)
type ElementQuads struct {
	QuadVertices    []float32
	QuadTexCoords   []uint8
	QuadIndices     []uint16
	OffsetVertices  int
	OffsetTexCoords int
	OffsetIndices   int

	// this is total bytes required for VBO buffer
	// e.g. ContextScreen will add up bytes for both QuadVertices + QuadTexCoords.
	//      ContextFramebuffer will add up bytes for QuadVertices + QuadTexCoords + QuadColors.
	BytesTotal int

	// QuadColors is only used by ContextFramebuffer
	QuadColors   []uint32
	OffsetColors int
}

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
	window, err := glfw.CreateWindow(windowWidth, windowHeight, "Quad 3D", nil, nil)
	if err != nil {
		panic(err)
	}
	window.MakeContextCurrent()

	// pixel dimension and texel dimensions are not the same in high resolution monitors
	// so we must account for that in many of the functions we use.
	// e.g. gl.Viewport, gl.Scissor, gl.ReadPixels, gl.LineWidth, gl.RenderbufferStorage, and gl.TexImage2D
	dpiScaleX, dpiScaleY = window.GetContentScale()

	// ensure framebuffer and screen uses maximum window size
	window.SetFramebufferSizeCallback(fboSizeCallback)
	window.SetSizeCallback(fboSizeCallback)

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

// on window size change (by OS or user resize) this callback executes
func fboSizeCallback(_ *glfw.Window, width int, height int) {
	// TODO: test this function
	panic("framebufferSizeCallback")
	// make sure the viewport matches the new window dimensions; note that width and
	// height will be significantly larger than specified on retina displays.
	gl.Viewport(0, 0, int32(width), int32(height))
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

	// prepare screen program and buffers (vbo, ibo)
	ctxScreen.setupProgram()
	ctxScreen.setupBuffers()

	// prepare framebuffer program and buffers (vbo, ibo, fbo) and camera
	ctxFramebuffer.setupProgram()
	ctxFramebuffer.setupBuffers()
	ctxFramebuffer.setupCamera(90, mgl32.Vec3{0, 0, 0.5}, mgl32.Vec3{0.1, 0.1, -1})

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

func makeQuadIndices(quadVerticesLen int) []uint16 {
	rectangleCount := quadVerticesLen / (verticesPerQuad * vertexPositionSize)
	i := uint16((rectangleCount - 1)) * verticesPerQuad
	return []uint16{
		i, i + 1, i + 2, // first triangle
		i, i + 2, i + 3, // second triangle
	}
}

func (q *ElementQuads) DebugPrint() {
	fmt.Printf("RECT_COUNT -- Rectangles: %v\n", len(q.QuadIndices)/indicesPerQuad)
	fmt.Printf("RAW_LENGTH -- Rectangle has %v vertex\nVertices   %v (%v-per-vertex)\nTexCoord   %v (%v-per-vertex)\nColors     %v (%v-per-vertex)\nIndices    %v (%v-per-rectangle)\n", verticesPerQuad, len(q.QuadVertices), vertexPositionSize, len(q.QuadTexCoords), vertexTexCoordSize, len(q.QuadColors), vertexColorSize, len(q.QuadIndices), indicesPerQuad)
}

func (q *ElementQuads) DrawRectangle(w float32, h float32, z float32, clr color.Color) {
	q.QuadVertices = append(q.QuadVertices, makeQuadVertices(w, h, z)...)
	q.QuadTexCoords = append(q.QuadTexCoords, makeQuadTextureCoord()...)
	q.QuadColors = append(q.QuadColors, makeQuadColors(clr.RGBA())...)
	q.QuadIndices = append(q.QuadIndices, makeQuadIndices(len(q.QuadVertices))...)
}

func load() {
	ctxScreen.load()
	ctxFramebuffer.load()
}

func (ctx *ContextScreen) load() {

	// initalize screen quads
	ctx.quads = &ElementQuads{
		QuadVertices:    []float32{},
		QuadTexCoords:   []uint8{},
		QuadIndices:     []uint16{},
		OffsetVertices:  0,
		OffsetTexCoords: 0,
		OffsetIndices:   0,
		BytesTotal:      0, // will be calculated to the total bytes needed for VBO buffer (QuadVertices + QuadTexCoords)
	}

	// TODO: make makeQuadVertices more generalized by introducing x,y,z positions as well as width, height values.
	// a single quad to cover entire screen in white
	//ctx.quads.QuadVertices = append(ctx.quads.QuadVertices, makeQuadVertices(1, 1, 0)...) // z-depth does not matter, we disable DEPTH_TEST for "real screen"
	ctx.quads.QuadVertices = []float32{
		1, 1, 0, // v0 position = top-right
		-1, 1, 0, // v1 position = top-left
		-1, -1, 0, // v2 position = bottom-left
		1, -1, 0, // v3 position = bottom-right
	}
	ctx.quads.QuadTexCoords = append(ctx.quads.QuadTexCoords, makeQuadTextureCoord()...)
	ctx.quads.QuadIndices = append(ctx.quads.QuadIndices, makeQuadIndices(len(ctx.quads.QuadVertices))...)

}

func (ctx *ContextFramebuffer) load() {

	// initalize framebuffer quads
	ctx.quads = &ElementQuads{
		QuadVertices:    []float32{},
		QuadTexCoords:   []uint8{},
		QuadIndices:     []uint16{},
		OffsetVertices:  0,
		OffsetTexCoords: 0,
		OffsetIndices:   0,
		BytesTotal:      0, // will be calculated to the total bytes needed for VBO buffer (QuadVertices + QuadTexCoords + QuadColors)
		QuadColors:      []uint32{},
		OffsetColors:    0,
	}

	// draw red rectangle
	ctx.quads.DrawRectangle(2, 2, -1.2, color.NRGBA{1, 0, 0, 1})

	// draw blue rectangle
	ctx.quads.DrawRectangle(1, 1, -1.1, color.NRGBA{0, 0, 1, 1})

	// print debug info for shapes
	ctx.quads.DebugPrint()

}

func draw() {

	// bind proxy offscreen (framebuffer) and draw elements
	ctxFramebuffer.bind()
	ctxFramebuffer.draw()

	// bind real screen and draw rasterized texture (output from framebuffer)
	// in other words, using the proxy screen's rendered image, overlay ontop real screen using a single quad
	ctxScreen.bind()
	ctxScreen.draw()

	// check for accumulated OpenGL errors
	checkGLError()

}

// use proxy offscreen for rendering using framebuffers
func (ctx *ContextFramebuffer) bind() {

	// bind Framebuffer program
	gl.UseProgram(ctx.program)

	// bind proxy framebuffer instead of default framebuffer
	gl.BindFramebufferEXT(gl.FRAMEBUFFER_EXT, ctx.fbo)

	// clear proxy screen to gray
	gl.ClearColor(0.5, 0.5, 0.5, 1) // TODO: can set this once during creation instead of each bind
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	// ensure depth test is enabled during proxy screen usage
	gl.Enable(gl.DEPTH_TEST)

}

// use default (real) screen for rendering
func (ctx *ContextScreen) bind() {

	// bind Screen program
	gl.UseProgram(ctx.program)

	// unbind proxy framebuffer and set back to default framebuffer
	gl.BindFramebufferEXT(gl.FRAMEBUFFER_EXT, 0)

	// clear screen to black
	gl.ClearColor(0, 0, 0, 1)     // TODO: can set this once during creation instead of each bind
	gl.Clear(gl.COLOR_BUFFER_BIT) // no need to clear depth, we will disable depth

	// disable depth test
	gl.Disable(gl.DEPTH_TEST)

}

func (ctx *ContextFramebuffer) draw() {

	// gl.Begin()
	gl.BindBuffer(gl.ARRAY_BUFFER, ctx.vbo)              // bind vertex buffer
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, ctx.ibo)      // bind indices buffer
	gl.EnableVertexAttribArray(ctx.attribVertexPosition) // enable vertex position
	gl.EnableVertexAttribArray(ctx.attribVertexTexCoord) // enable vertex texture coordinate
	gl.EnableVertexAttribArray(ctx.attribVertexColor)    // enable vertex color

	// configure and enable vertex position
	gl.VertexAttribPointer(ctx.attribVertexPosition, vertexPositionSize, gl.FLOAT, false, 0, gl.PtrOffset(ctx.quads.OffsetVertices))

	// configure and enable vertex texture coordinate
	gl.VertexAttribPointer(ctx.attribVertexTexCoord, vertexTexCoordSize, gl.UNSIGNED_BYTE, false, 0, gl.PtrOffset(ctx.quads.OffsetTexCoords))

	// configure and enable vertex color
	gl.VertexAttribPointer(ctx.attribVertexColor, vertexColorSize, gl.UNSIGNED_INT, false, 0, gl.PtrOffset(ctx.quads.OffsetColors))

	// draw rectangles
	gl.DrawElements(gl.TRIANGLES, int32(len(ctx.quads.QuadIndices)), gl.UNSIGNED_SHORT, gl.PtrOffset(ctx.quads.OffsetIndices))

	// gl.End()
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)                     // unbind vertex buffer
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, 0)             // unbind indices buffer
	gl.DisableVertexAttribArray(ctx.attribVertexPosition) // disable vertex position
	gl.DisableVertexAttribArray(ctx.attribVertexTexCoord) // disable vertex texture coordinate
	gl.DisableVertexAttribArray(ctx.attribVertexColor)    // disable vertex color

}

func (ctx *ContextScreen) draw() {

	// gl.Begin()
	gl.BindBuffer(gl.ARRAY_BUFFER, ctx.vbo)                  // bind vertex buffer
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, ctx.ibo)          // bind indices buffer
	gl.BindTexture(gl.TEXTURE_2D, ctxFramebuffer.fboTexture) // bind shared texture from Framebuffer-FBO (proxy screen) to Screen-FBO (real screen)
	gl.EnableVertexAttribArray(ctx.attribVertexPosition)     // enable vertex position
	gl.EnableVertexAttribArray(ctx.attribVertexTexCoord)     // enable vertex texture coordinate

	// configure and enable vertex position
	gl.VertexAttribPointer(ctx.attribVertexPosition, vertexPositionSize, gl.FLOAT, false, 0, gl.PtrOffset(ctx.quads.OffsetVertices))

	// configure and enable vertex texture coordinate
	gl.VertexAttribPointer(ctx.attribVertexTexCoord, vertexTexCoordSize, gl.UNSIGNED_BYTE, false, 0, gl.PtrOffset(ctx.quads.OffsetTexCoords))

	// draw rectangles
	gl.DrawElements(gl.TRIANGLES, int32(len(ctx.quads.QuadIndices)), gl.UNSIGNED_SHORT, gl.PtrOffset(ctx.quads.OffsetIndices))

	// gl.End()
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)                     // unbind vertex buffer
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, 0)             // unbind indices buffer
	gl.BindTexture(gl.TEXTURE_2D, 0)                      // unbind texture
	gl.DisableVertexAttribArray(ctx.attribVertexPosition) // disable vertex position
	gl.DisableVertexAttribArray(ctx.attribVertexTexCoord) // disable vertex texture coordinate

}

func (ctx *ContextScreen) setupBuffers() {

	// use SCREEN program
	gl.UseProgram(ctx.program)

	// unbind FBO
	gl.BindFramebufferEXT(gl.FRAMEBUFFER_EXT, 0)

	// to be more efficient, vertices position are in float32 and texture coordinate in uint8
	ctx.quads.BytesTotal = (len(ctx.quads.QuadVertices) * bytesFloat32) + (len(ctx.quads.QuadTexCoords) * bytesUint8)

	// vbo data offsets
	ctx.quads.OffsetVertices = 0 * bytesFloat32
	ctx.quads.OffsetTexCoords = ctx.quads.OffsetVertices + len(ctx.quads.QuadVertices)*bytesFloat32

	// ibo data offsets
	ctx.quads.OffsetIndices = 0 * bytesUint16

	// create VBOs
	gl.GenBuffers(1, &ctx.vbo) // buffer for vertex position and texture coordinate
	gl.GenBuffers(1, &ctx.ibo) // buffer for vertex indices

	// copy vertex data to VBO
	gl.BindBuffer(gl.ARRAY_BUFFER, ctx.vbo)
	gl.BufferData(gl.ARRAY_BUFFER, ctx.quads.BytesTotal, nil, gl.STATIC_DRAW)                                                              // initalize but do not copy any data
	gl.BufferSubData(gl.ARRAY_BUFFER, ctx.quads.OffsetVertices, len(ctx.quads.QuadVertices)*bytesFloat32, gl.Ptr(ctx.quads.QuadVertices))  // copy vertices starting from 0 offest
	gl.BufferSubData(gl.ARRAY_BUFFER, ctx.quads.OffsetTexCoords, len(ctx.quads.QuadTexCoords)*bytesUint8, gl.Ptr(ctx.quads.QuadTexCoords)) // copy textures after vertices
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)

	// copy index data to VBO
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, ctx.ibo)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(ctx.quads.QuadIndices)*bytesUint16, gl.Ptr(ctx.quads.QuadIndices), gl.STATIC_DRAW)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, 0)

	// unbind SCREEN program
	gl.UseProgram(0)

}

// https://en.wikipedia.org/wiki/Vertex_buffer_object
// https://www.songho.ca/opengl/gl_vbo.html#create
// https://learnopengl.com/Advanced-OpenGL/Framebuffers
func (ctx *ContextFramebuffer) setupBuffers() {

	// use PROXY program
	gl.UseProgram(ctx.program)

	// to be more efficient, vertices position are in float32, texture coordinate in uint8, and color is in uint32
	ctx.quads.BytesTotal = (len(ctx.quads.QuadVertices) * bytesFloat32) + (len(ctx.quads.QuadTexCoords) * bytesUint8) + (len(ctx.quads.QuadColors) * bytesUint32)

	// vbo data offsets
	ctx.quads.OffsetVertices = 0 * bytesFloat32
	ctx.quads.OffsetTexCoords = ctx.quads.OffsetVertices + len(ctx.quads.QuadVertices)*bytesFloat32
	ctx.quads.OffsetColors = ctx.quads.OffsetTexCoords + len(ctx.quads.QuadTexCoords)*bytesUint8

	// ibo data offsets
	ctx.quads.OffsetIndices = 0 * bytesUint16

	// create FBO and bind to it
	gl.GenFramebuffersEXT(1, &ctx.fbo) // offscreen rendering use framebuffer extension
	gl.BindFramebufferEXT(gl.FRAMEBUFFER_EXT, ctx.fbo)

	// attach texture to FBO (color buffer component)
	ctx.attachTexture()

	/// attach renderbuffer to FBO (combined depth and stencil buffer component)
	ctx.attachRenderbuffer()

	// check if FBO is ready and valid
	if gl.CheckFramebufferStatusEXT(gl.FRAMEBUFFER_EXT) != gl.FRAMEBUFFER_COMPLETE_EXT {
		panic("Framebuffer (FBO) FATAL ERROR")
	}

	// create VBOs
	gl.GenBuffers(1, &ctx.vbo) // buffer for vertex position, texture coordinate, and color
	gl.GenBuffers(1, &ctx.ibo) // buffer for vertex indices

	// copy vertex data to VBO
	gl.BindBuffer(gl.ARRAY_BUFFER, ctx.vbo)
	gl.BufferData(gl.ARRAY_BUFFER, ctx.quads.BytesTotal, nil, gl.STATIC_DRAW)                                                              // initalize but do not copy any data
	gl.BufferSubData(gl.ARRAY_BUFFER, ctx.quads.OffsetVertices, len(ctx.quads.QuadVertices)*bytesFloat32, gl.Ptr(ctx.quads.QuadVertices))  // copy vertices starting from 0 offest
	gl.BufferSubData(gl.ARRAY_BUFFER, ctx.quads.OffsetTexCoords, len(ctx.quads.QuadTexCoords)*bytesUint8, gl.Ptr(ctx.quads.QuadTexCoords)) // copy textures after vertices
	gl.BufferSubData(gl.ARRAY_BUFFER, ctx.quads.OffsetColors, len(ctx.quads.QuadColors)*bytesUint32, gl.Ptr(ctx.quads.QuadColors))         // copy colors after textures
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)

	// copy index data to VBO
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, ctx.ibo)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(ctx.quads.QuadIndices)*bytesUint16, gl.Ptr(ctx.quads.QuadIndices), gl.STATIC_DRAW)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, 0)

	// unbind FBO
	gl.BindFramebufferEXT(gl.FRAMEBUFFER_EXT, 0)

	// unbind PROXY program
	gl.UseProgram(0)

}

// http://www.songho.ca/opengl/gl_fbo.html
func (ctx *ContextFramebuffer) attachTexture() {

	// create texture for framebuffer attachment, and bind to it
	// NOTE: a texture can be attached to multiple FBOs, where its image storage is shared
	//       this is an important, we use it to render the final drawn texture from Framebuffer-FBO to Screen-FBO.
	gl.GenTextures(1, &ctx.fboTexture)
	gl.BindTexture(gl.TEXTURE_2D, ctx.fboTexture)

	// initalize texture (memory space and min/mag filters)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGB, windowWidth*int32(dpiScaleX), windowHeight*int32(dpiScaleY), 0, gl.RGB, gl.UNSIGNED_BYTE, nil)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)

	// attach texture to framebuffer
	gl.FramebufferTexture2DEXT(gl.FRAMEBUFFER_EXT, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, ctx.fboTexture, 0)

	// NOTE: do not unbind texture for this framebuffer
	// remember, each "screen" has its own state machine, and therefore
	// we can leave this texture always binded

}

// http://www.songho.ca/opengl/gl_fbo.html
func (ctx *ContextFramebuffer) attachRenderbuffer() {

	// create renderbuffer for depth and stencil testing. and bind to it
	gl.GenRenderbuffersEXT(1, &ctx.fboRenderbuffer)
	gl.BindRenderbufferEXT(gl.RENDERBUFFER_EXT, ctx.fboRenderbuffer)

	// initalize renderbuffer memory space
	gl.RenderbufferStorageEXT(gl.RENDERBUFFER_EXT, gl.DEPTH24_STENCIL8, windowWidth*int32(dpiScaleX), windowHeight*int32(dpiScaleY))

	// unbind renderbuffer
	gl.BindRenderbufferEXT(gl.RENDERBUFFER_EXT, 0)

	// attach renderbuffer to framebuffer
	gl.FramebufferRenderbufferEXT(gl.FRAMEBUFFER_EXT, gl.DEPTH_STENCIL_ATTACHMENT, gl.RENDERBUFFER_EXT, ctx.fboRenderbuffer)

}

func (ctx *ContextScreen) setupProgram() {

	var err error

	// configure program, load shaders, and link attributes
	ctx.program, err = newProgram(vertexShaderScreen, fragmentShaderScreen)
	if err != nil {
		panic(err)
	}
	gl.UseProgram(ctx.program)

	// get attribute index for later use
	ctx.attribVertexPosition = uint32(gl.GetAttribLocation(ctx.program, gl.Str("vertexPosition\x00")))
	ctx.attribVertexTexCoord = uint32(gl.GetAttribLocation(ctx.program, gl.Str("vertexTexCoord\x00")))

	// debug print
	fmt.Printf("attribVertexPosition: %v attribVertexTexCoord: %v\n", ctx.attribVertexPosition, ctx.attribVertexTexCoord)

	// unbind program
	gl.UseProgram(0)

}

func (ctx *ContextFramebuffer) setupProgram() {

	var err error

	// configure program, load shaders, and link attributes
	ctx.program, err = newProgram(vertexShaderFramebuffer, fragmentShaderFramebuffer)
	if err != nil {
		panic(err)
	}
	gl.UseProgram(ctx.program)

	// get attribute index for later use
	ctx.attribVertexPosition = uint32(gl.GetAttribLocation(ctx.program, gl.Str("vertexPosition\x00")))
	ctx.attribVertexTexCoord = uint32(gl.GetAttribLocation(ctx.program, gl.Str("vertexTexCoord\x00")))
	ctx.attribVertexColor = uint32(gl.GetAttribLocation(ctx.program, gl.Str("vertexColor\x00")))

	// debug print
	fmt.Printf("attribVertexPosition: %v attribVertexTexCoord: %v attribVertexColor: %v\n", ctx.attribVertexPosition, ctx.attribVertexTexCoord, ctx.attribVertexColor)

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
func (ctx *ContextFramebuffer) setupCamera(fov float32, cameraposition mgl32.Vec3, target mgl32.Vec3) {

	// use PROXY program
	gl.UseProgram(ctx.program)

	// CREATE (PRESPECTIVE) PROJECTION MATRIX
	// a matrix to transform from eye to NDC coordinates
	projection := mgl32.Perspective(mgl32.DegToRad(fov), float32(windowWidth*dpiScaleX)/float32(windowHeight*dpiScaleY), 0.1, 10.0)
	projectionUniform := gl.GetUniformLocation(ctx.program, gl.Str("projection\x00"))
	gl.UniformMatrix4fv(projectionUniform, 1, false, &projection[0])

	// CREATE (CAMERA) VIEW MATRIX
	// a matrix to transform from eye to NDC coordinates
	camera := mgl32.LookAtV(cameraposition, target, mgl32.Vec3{0, 1, 0})
	cameraUniform := gl.GetUniformLocation(ctx.program, gl.Str("camera\x00"))
	gl.UniformMatrix4fv(cameraUniform, 1, false, &camera[0])

	// CREATE (OBJECT) MODEL MATRIX
	// a matrix to transform from object to eye coordinates
	model := mgl32.Ident4()
	modelUniform := gl.GetUniformLocation(ctx.program, gl.Str("model\x00"))
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
	gl_FragColor = fragmentColor;
}
` + "\x00"

var vertexShaderScreen = `
#version 120

// input
attribute vec2 vertexPosition; // z-axis discarded
attribute vec2 vertexTexCoord;

// output
varying vec2 fragmentTexCoord;

void main() {
	fragmentTexCoord = vertexTexCoord;
	gl_Position = vec4(vertexPosition, 0, 1);
}
` + "\x00"

var fragmentShaderScreen = `
#version 120

// input
uniform sampler2D screenTexture;

// input
varying vec2 fragmentTexCoord;

void main() {
	gl_FragColor = texture2D(screenTexture, fragmentTexCoord);
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
