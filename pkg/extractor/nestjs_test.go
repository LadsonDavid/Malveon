package extractor

import (
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

const nestPkg = `{"dependencies":{"@nestjs/core":"^10","@nestjs/common":"^10"}}`

func TestNestControllerPrefixAndMethodDecorators(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": nestPkg,
		"src/cats/cats.controller.ts": `
import { Controller, Get, Post, Delete, Param, Body, All, Options, Head } from '@nestjs/common'

@Controller('cats')
export class CatsController {
  constructor(private readonly catsService: CatsService) {}

  @Post()
  create(@Body() dto: CreateCatDto) { return this.catsService.create(dto) }

  @Get()
  findAll() { return [] }

  @Get(':id')
  findOne(@Param('id') id: string) { return {} }

  @Delete(':id')
  remove(@Param('id') id: string) { return {} }

  @Get(['breed', 'kind'])
  both() {}

  @All('any') anyMethod() {}
  @Head('h') h() {}
  @Options('o') o() {}
}`,
	})
	wantRoutes(t, g,
		"POST /cats", "GET /cats", "GET /cats/:id", "DELETE /cats/:id",
		"GET /cats/breed", "GET /cats/kind", "ANY /cats/any", "HEAD /cats/h", "OPTIONS /cats/o")
	for _, n := range g.Nodes {
		if n.Kind == graph.RouteHandler && n.Unmounted {
			t.Errorf("a plain controller has a known URL: %+v", n)
		}
	}
}

func TestNestControllerWithoutPrefixAndArrayPrefix(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": nestPkg,
		"src/app.controller.ts": `
import { Controller, Get } from '@nestjs/common'
@Controller()
export class AppController {
  @Get('health') health() {}
}
@Controller(['v1/users', 'users'])
export class UsersController {
  @Get() list() {}
}`,
	})
	wantRoutes(t, g, "GET /health", "GET /v1/users", "GET /users")
}

func TestNestTwoControllersInOneFileKeepTheirOwnRoutes(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": nestPkg,
		"src/two.controller.ts": `
import { Controller, Get } from '@nestjs/common'
@Controller('a')
export class A { @Get('x') x() { if (true) { return 1 } } }
@Controller('b')
export class B { @Get('y') y() {} }`,
	})
	wantRoutes(t, g, "GET /a/x", "GET /b/y")
	wantNoRoute(t, g, "GET /a/y")
	wantNoRoute(t, g, "GET /b/x")
}

func TestNestGlobalPrefix(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": nestPkg,
		"src/main.ts": `
import { NestFactory } from '@nestjs/core'
async function bootstrap() {
  const app = await NestFactory.create(AppModule)
  app.setGlobalPrefix('api')
  await app.listen(3000)
}`,
		"src/cats.controller.ts": `
import { Controller, Get } from '@nestjs/common'
@Controller('cats')
export class CatsController { @Get() findAll() {} }`,
	})
	wantRoutes(t, g, "GET /api/cats")
}

func TestNestUnreadableGlobalPrefixMeansUnknownUrl(t *testing.T) {
	for name, call := range map[string]string{
		"variable": `app.setGlobalPrefix(process.env.PREFIX)`,
		"exclude":  `app.setGlobalPrefix('api', { exclude: ['health'] })`,
	} {
		g := extractProject(t, map[string]string{
			"package.json": nestPkg,
			"src/main.ts":  "import { NestFactory } from '@nestjs/core'\nconst app = await NestFactory.create(M)\n" + call + "\n",
			"src/c.controller.ts": `
import { Controller, Get } from '@nestjs/common'
@Controller('cats')
export class C { @Get() f() {} }`,
		})
		found := false
		for _, n := range g.Nodes {
			if n.Kind == graph.RouteHandler {
				found = true
				if !n.Unmounted {
					t.Errorf("%s: the real URL is unknown, the route must be Unmounted: %+v", name, n)
				}
			}
		}
		if !found {
			t.Errorf("%s: no route found", name)
		}
	}
}

func TestNestUriVersioning(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": nestPkg,
		"src/main.ts": `
import { NestFactory } from '@nestjs/core'
import { VersioningType } from '@nestjs/common'
const app = await NestFactory.create(M)
app.setGlobalPrefix('api')
app.enableVersioning({ type: VersioningType.URI, defaultVersion: '1' })`,
		"src/c.controller.ts": `
import { Controller, Get, Version, VERSION_NEUTRAL } from '@nestjs/common'
@Controller('cats')
export class C {
  @Get() base() {}
  @Version('2') @Get('new') fresh() {}
  @Version(['3', '4'])
  @Get('multi') multi() {}
  @Version(VERSION_NEUTRAL) @Get('neutral') neutral() {}
}
@Controller({ path: 'dogs', version: '5' })
export class D { @Get() all() {} }`,
	})
	wantRoutes(t, g,
		"GET /api/v1/cats", "GET /api/v2/cats/new", "GET /api/v3/cats/multi", "GET /api/v4/cats/multi",
		"GET /api/cats/neutral", "GET /api/v5/dogs")
}

func TestNestHeaderVersioningDoesNotChangeTheUrl(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": nestPkg,
		"src/main.ts": `
import { NestFactory } from '@nestjs/core'
const app = await NestFactory.create(M)
app.enableVersioning({ type: VersioningType.HEADER, header: 'X-V' })`,
		"src/c.controller.ts": `
import { Controller, Get } from '@nestjs/common'
@Controller({ path: 'cats', version: '1' })
export class C { @Get() f() {} }`,
	})
	wantRoutes(t, g, "GET /cats")
}

func TestNestRouterModuleMeansUnknownUrls(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": nestPkg,
		"src/app.module.ts": `
import { RouterModule } from '@nestjs/core'
@Module({ imports: [RouterModule.register([{ path: 'admin', module: AdminModule }])] })
export class AppModule {}`,
		"src/c.controller.ts": `
import { Controller, Get } from '@nestjs/common'
@Controller('users')
export class C { @Get() f() {} }`,
	})
	for _, n := range g.Nodes {
		if n.Kind == graph.RouteHandler && !n.Unmounted {
			t.Errorf("with RouterModule the prefix is unknown: %+v", n)
		}
	}
}

func TestNestWildcardsAndOptionalParams(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": nestPkg,
		"src/c.controller.ts": `
import { Controller, Get } from '@nestjs/common'
@Controller('files')
export class C {
  @Get('all/*') all() {}
  @Get('name/:name?') maybe() {}
  @Get('rx/(ab)+c') rx() {}
}`,
	})
	wantRoutes(t, g, "GET /files/all/:...splat?", "GET /files/name", "GET /files/name/:name", "GET  ?")
}

func TestDecoratorsOutsideNestAreNotRoutes(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": `{"dependencies":{"express":"^4"}}`,
		"src/a.ts": `
@Controller('x')
class Foo { @Get('y') bar() {} }`,
	})
	if got := routesOf(g); len(got) != 0 {
		t.Errorf("no @nestjs import and no Nest dependency: %v", got)
	}
}

func TestNestDecoratorInACommentIsIgnored(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": nestPkg,
		"src/c.controller.ts": `
import { Controller, Get } from '@nestjs/common'
@Controller('cats')
export class C {
  // @Get('old')
  /* @Get('older') */
  @Get('new') f() {}
}`,
	})
	wantRoutes(t, g, "GET /cats/new")
	wantNoRoute(t, g, "GET /cats/old")
	wantNoRoute(t, g, "GET /cats/older")
}

func TestNestCommentsWithApostrophesDoNotHideRoutes(t *testing.T) {
	g := extractProject(t, map[string]string{
		"package.json": nestPkg,
		"src/c.controller.ts": `
import { Controller, Get, Post } from '@nestjs/common'
@Controller('cats')
export class C {
  // the user's cats, don't cache
  @Get() list() { return [] }

  /* it's created here */
  @Post() create() { return {} }
}`,
	})
	wantRoutes(t, g, "GET /cats", "POST /cats")
}
