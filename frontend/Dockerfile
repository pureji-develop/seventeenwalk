# SeventeenWalk frontend — local Vite (source mounted via compose)
FROM node:20-alpine
WORKDIR /app
COPY package.json ./
RUN npm install
ENV VITE_PROXY_TARGET=http://backend:8080
ENV VITE_PORT=5175
EXPOSE 5175
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0", "--port", "5175"]
