# Build the Vue SPA, then serve the static bundle via nginx (which also proxies
# /xchats to the backend so the app and API are same-origin).
# Keep this major in sync with /.nvmrc — ci.yml's "node version pins agree"
# step fails the build if they drift. Track Node LTS lines only (24 = Krypton).
FROM node:24 AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:1.30-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
