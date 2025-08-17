function greetUser(name: string): string {
    return `Hello, ${name}!`;
}

class User {
    constructor(public name: string) {}
    
    greet(): string {
        return greetUser(this.name);
    }
}

const user = new User("World");
console.log(user.greet());
