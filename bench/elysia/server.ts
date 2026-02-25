import { Elysia } from 'elysia'

const port = parseInt(process.env.PORT || '3001')

const app = new Elysia()
  .get('/', () => 'Hello, World!')
  .get('/users/:id', ({ params: { id } }) => id)
  .post('/json', ({ body }) => body)
  .listen(port)

console.log(`Elysia server running on port ${port}`)