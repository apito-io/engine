#!/usr/bin/env node

/**
 * Comprehensive Hello World JavaScript Plugin - SDK Version
 * 
 * This plugin demonstrates the full capabilities of the Apito JavaScript Plugin SDK,
 * providing feature parity with the Go hello-world plugin.
 * 
 * Features:
 * - GraphQL Queries with complex object types
 * - GraphQL Mutations with nested data
 * - Comprehensive REST APIs (CRUD operations)
 * - Custom functions
 * - Object type definitions
 * - Pagination and filtering
 * 
 * @version 2.0.3
 * @author Apito Engine Team
 */

const { 
    init, 
    // GraphQL field helpers
    StringField, 
    IntField,
    BooleanField,
    FloatField,
    ListField,
    ObjectField,
    FieldWithArgs, 
    // GraphQL argument helpers
    StringArg, 
    IntArg,
    BooleanArg,
    FloatArg,
    ObjectArg,
    ListArg,
    // Type system helpers
    createObjectType,
    NewObjectType,
    // REST endpoint helpers
    GETEndpoint, 
    POSTEndpoint,
    PUTEndpoint,
    DELETEEndpoint,
    // Schema helpers
    ObjectSchema, 
    StringSchema,
    IntegerSchema,
    BooleanSchema,
    ArraySchema,
    // Utility functions
    getStringArg,
    getIntArg,
    getBoolArg,
    getFloatArg,
    getObjectArg,
    getArrayArg,
    getPathParam,
    getQueryParam,
    getBodyParam,
    logRESTArgs
} = require('@apito-io/js-apito-plugin-sdk');

async function main() {
    process.stderr.write('🌍 Comprehensive Hello World JS Plugin: Starting with SDK v2.1.0...\n');
    
    // Initialize the plugin with detailed information
    const plugin = init('hc-hello-world-js-plugin', '2.1.0-sdk', 'apito-plugin-key');

    // ========================================
    // DEFINE SIMPLE OBJECT TYPES (AVAILABLE METHODS ONLY)
    // ========================================
    
    process.stderr.write('📋 Defining object types...\n');

    // Simple User object type using available methods
    const userType = NewObjectType('User', 'A user in the system')
        .addStringField('id', 'User ID', false)
        .addStringField('name', 'User\'s full name', false)
        .addStringField('email', 'User\'s email address', true)
        .addStringField('username', 'User\'s username', true)
        .addBooleanField('active', 'Whether the user is active', false)
        .addStringField('createdAt', 'When the user was created', true)
        .build();

    // Simple Product object type
    const productType = NewObjectType('Product', 'A product in our catalog')
        .addStringField('id', 'Product ID', false)
        .addStringField('name', 'Product name', false)
        .addStringField('description', 'Product description', true)
        .addFloatField('price', 'Product price', false)
        .addIntField('stock', 'Stock quantity', false)
        .build();

    // Simple Task object type
    const taskType = NewObjectType('Task', 'A simple task object')
        .addStringField('id', 'Task ID', false)
        .addStringField('title', 'Task title', false)
        .addStringField('status', 'Task status', false)
        .addBooleanField('completed', 'Whether task is completed', false)
        .addStringField('createdAt', 'When task was created', true)
        .build();

    // ========================================
    // REGISTER GRAPHQL QUERIES
    // ========================================
    
    process.stderr.write('📋 Registering GraphQL queries...\n');

    // Simple query with complex arguments (matches Go version)
    plugin.registerQuery('helloWorldQueryFahim',
        FieldWithArgs(StringField('Hello World Plugin Query with Arguments'), {
            'name': StringArg('Name to greet (optional)'),
            'object': ObjectArg('Object argument', {
                'name': StringArg('Object name'),
                'age': IntArg('Object age')
            }),
            'arrayofObjects': ListArg(ObjectArg('Object in array', {
                'id': StringArg('Object ID'),
                'name': StringArg('Object name')
            }), 'Array of objects')
        }),
        helloWorldResolver
    );

    process.stderr.write('📋 DEBUG: helloWorldQueryFahim field definition: ' + JSON.stringify(FieldWithArgs(StringField('Hello World Plugin Query with Arguments'), {
        'name': StringArg('Name to greet (optional)'),
        'object': ObjectArg('Object argument', {
            'name': StringArg('Object name'),
            'age': IntArg('Object age')
        }),
        'arrayofObjects': ListArg(ObjectArg('Object in array', {
            'id': StringArg('Object ID'),
            'name': StringArg('Object name')
        }), 'Array of objects')
    }), null, 2) + '\n');

    // Query that returns a single User object
    plugin.registerQuery('getUserProfile',
        FieldWithArgs(ObjectField('User profile information', userType), {
            'userId': StringArg('User ID to fetch')
        }),
        getUserProfileResolver
    );

    process.stderr.write('📋 DEBUG: getUserProfile field definition: ' + JSON.stringify(FieldWithArgs(ObjectField('User profile information', userType), {
        'userId': StringArg('User ID to fetch')
    }), null, 2) + '\n');

    // Query that returns an array of User objects
    plugin.registerQuery('getUsers',
        FieldWithArgs(ListField(userType, 'List of users'), {
            'limit': IntArg('Maximum number of users to return'),
            'offset': IntArg('Number of users to skip'),
            'filter': StringArg('Filter users by name or email')
        }),
        getUsersResolver
    );

    process.stderr.write('📋 DEBUG: getUsers field definition: ' + JSON.stringify(FieldWithArgs(ListField(userType, 'List of users'), {
        'limit': IntArg('Maximum number of users to return'),
        'offset': IntArg('Number of users to skip'),
        'filter': StringArg('Filter users by name or email')
    }), null, 2) + '\n');

    // Query that returns a single product
    plugin.registerQuery('getProduct',
        FieldWithArgs(ObjectField('Product information', productType), {
            'productId': StringArg('Product ID to fetch')
        }),
        getProductResolver
    );

    process.stderr.write('📋 DEBUG: getProduct field definition: ' + JSON.stringify(FieldWithArgs(ObjectField('Product information', productType), {
        'productId': StringArg('Product ID to fetch')
    }), null, 2) + '\n');

    // Query that returns a paginated list of products
    plugin.registerQuery('getProductsPaginated',
        FieldWithArgs(StringField('Get paginated list of products (JSON string)'), {
            'page': IntArg('Page number (1-based)'),
            'pageSize': IntArg('Number of items per page'),
            'category': StringArg('Filter by category'),
            'minPrice': FloatArg('Minimum price'),
            'maxPrice': FloatArg('Maximum price')
        }),
        getProductsPaginatedResolver
    );

    // Query that returns an array of tasks
    plugin.registerQuery('getTasks',
        FieldWithArgs(StringField('Get a list of tasks (JSON string)'), {
            'userId': StringArg('User ID to get tasks for'),
            'status': StringArg('Filter by task status'),
            'completed': BooleanArg('Filter by completion status'),
            'limit': IntArg('Maximum number of tasks to return')
        }),
        getTasksResolver
    );

    // ========================================
    // REGISTER GRAPHQL MUTATIONS
    // ========================================
    
    process.stderr.write('📋 Registering GraphQL mutations...\n');

    plugin.registerMutation('createUser',
        FieldWithArgs(StringField('Create a new user (JSON response)'), {
            'input': ObjectArg('User creation data', {
                'name': StringArg('User\'s full name'),
                'email': StringArg('User\'s email address'),
                'username': StringArg('User\'s username')
            })
        }),
        createUserResolver
    );

    // Demonstrates array object argument functionality
    plugin.registerMutation('processBulkTags',
        FieldWithArgs(StringField('Process multiple tag objects result'), {
            'userId': StringArg('User ID to process tags for'),
            'tags': ListArg(ObjectArg('Tag object', {
                'tag_id': StringArg('Tag identifier'),
                'name': StringArg('Tag name'),
                'value': StringArg('Tag value'),
                'weight': FloatArg('Tag weight/importance'),
                'active': BooleanArg('Whether tag is active'),
                'metadata': StringArg('Additional metadata')
            }), 'Array of tag objects with structured data')
        }),
        processBulkTagsResolver
    );

    // ========================================
    // REGISTER REST APIS - Complete Examples
    // ========================================
    
    process.stderr.write('📋 Registering REST APIs...\n');

    // Simple GET endpoint without parameters
    plugin.registerRESTAPI(
        GETEndpoint('/hello', 'Simple hello endpoint').build(),
        helloRESTHandler
    );

    // POST endpoint with JSON body
    plugin.registerRESTAPI(
        POSTEndpoint('/custom-hello', 'Custom hello endpoint with POST data')
            .withRequestSchema(ObjectSchema({
                'name': StringSchema(),
                'message': StringSchema()
            }))
            .build(),
        customHelloRESTHandler
    );

    // GET endpoint with path parameters and query parameters
    plugin.registerRESTAPI(
        GETEndpoint('/users/:id', 'Get user by ID with query parameters')
            .withRequestSchema(ObjectSchema({
                'include_profile': BooleanSchema(),
                'format': StringSchema()
            }))
            .build(),
        getUserRESTHandler
    );

    // POST endpoint for creating resources
    plugin.registerRESTAPI(
        POSTEndpoint('/users', 'Create a new user')
            .withRequestSchema(ObjectSchema({
                'name': StringSchema(),
                'email': StringSchema(),
                'username': StringSchema(),
                'active': BooleanSchema()
            }))
            .build(),
        createUserRESTHandler
    );

    // PUT endpoint for updating resources
    plugin.registerRESTAPI(
        PUTEndpoint('/users/:id', 'Update user by ID')
            .withRequestSchema(ObjectSchema({
                'name': StringSchema(),
                'email': StringSchema(),
                'active': BooleanSchema()
            }))
            .build(),
        updateUserRESTHandler
    );

    // DELETE endpoint
    plugin.registerRESTAPI(
        DELETEEndpoint('/users/:id', 'Delete user by ID').build(),
        deleteUserRESTHandler
    );

    // GET endpoint with pagination
    plugin.registerRESTAPI(
        GETEndpoint('/users', 'List users with pagination')
            .withRequestSchema(ObjectSchema({
                'page': IntegerSchema(),
                'pageSize': IntegerSchema(),
                'search': StringSchema(),
                'active': BooleanSchema()
            }))
            .build(),
        listUsersRESTHandler
    );

    // POST endpoint with file upload simulation
    plugin.registerRESTAPI(
        POSTEndpoint('/upload', 'Handle file upload with metadata')
            .withRequestSchema(ObjectSchema({
                'filename': StringSchema(),
                'content': StringSchema(),
                'tags': ArraySchema(StringSchema()),
                'description': StringSchema()
            }))
            .build(),
        uploadFileRESTHandler
    );

    // GET endpoint for health check and metrics
    plugin.registerRESTAPI(
        GETEndpoint('/status', 'Plugin status and health metrics').build(),
        statusRESTHandler
    );

    // POST endpoint with complex nested data
    plugin.registerRESTAPI(
        POSTEndpoint('/complex-data', 'Handle complex nested JSON data')
            .withRequestSchema(ObjectSchema({
                'user': ObjectSchema({
                    'name': StringSchema(),
                    'email': StringSchema(),
                    'address': ObjectSchema({
                        'street': StringSchema(),
                        'city': StringSchema(),
                        'zip': StringSchema()
                    })
                }),
                'preferences': ArraySchema(StringSchema()),
                'metadata': ObjectSchema({})
            }))
            .build(),
        processComplexDataRESTHandler
    );

    // ========================================
    // REGISTER CUSTOM FUNCTIONS
    // ========================================
    
    process.stderr.write('📋 Registering custom functions...\n');

    plugin.registerFunction('customFunction', customFunction);

    // ========================================
    // START THE PLUGIN SERVER
    // ========================================
    
    process.stderr.write('🚀 Starting plugin server...\n');
    await plugin.serve();
}

// ========================================
// GRAPHQL RESOLVER IMPLEMENTATIONS
// ========================================

async function helloWorldResolver(context, args) {
    process.stderr.write('🚀 helloWorldResolver called with args: ' + JSON.stringify(args, null, 2) + '\n');
    process.stderr.write('🚀 helloWorldResolver context: ' + JSON.stringify(context, null, 2) + '\n');
    
    const name = getStringArg(args, 'name', 'World');
    const object = getObjectArg(args, 'object', {});
    const arrayOfObjects = getArrayArg(args, 'arrayofObjects', []);
    
    process.stderr.write('📊 Extracted data: ' + JSON.stringify({
        name,
        object,
        arrayOfObjects: arrayOfObjects.length + ' items'
    }) + '\n');

    let message = `Hello ${name}!`;
    
    if (object && object.name) {
        message += ` Object name: ${object.name}`;
        if (object.age) {
            message += `, age: ${object.age}`;
        }
    }
    
    if (arrayOfObjects && arrayOfObjects.length > 0) {
        message += ` Array contains ${arrayOfObjects.length} objects.`;
    }

    const result = {
        message: message,
        name: name,
        object: object,
        arrayOfObjects: arrayOfObjects,
        timestamp: new Date().toISOString()
    };

    process.stderr.write('🎯 helloWorldResolver returning: ' + JSON.stringify(result, null, 2) + '\n');
    return result;
}

async function getUserProfileResolver(context, args) {
    process.stderr.write('🔍 getUserProfileResolver called with args: ' + JSON.stringify(args, null, 2) + '\n');
    process.stderr.write('🔍 getUserProfileResolver context: ' + JSON.stringify(context, null, 2) + '\n');
    
    const userId = getStringArg(args, 'userId', '1');
    
    // Simulate database lookup
    const user = {
        id: userId,
        name: 'John Doe',
        email: 'john.doe@example.com',
        username: 'johndoe',
        address: {
            street: '123 Main St',
            city: 'Anytown',
            zip: '12345'
        },
        active: true,
        createdAt: '2023-01-01T00:00:00Z'
    };

    process.stderr.write('🎯 getUserProfileResolver returning: ' + JSON.stringify(user, null, 2) + '\n');
    return user;
}

async function getUsersResolver(context, args) {
    process.stderr.write('📋 getUsersResolver called with args: ' + JSON.stringify(args, null, 2) + '\n');
    process.stderr.write('📋 getUsersResolver context: ' + JSON.stringify(context, null, 2) + '\n');
    
    const limit = getIntArg(args, 'limit', 10);
    const offset = getIntArg(args, 'offset', 0);
    const filter = getStringArg(args, 'filter', '');
    
    process.stderr.write('🔍 DEBUG: Parsed arguments - limit: ' + limit + ', offset: ' + offset + ', filter: "' + filter + '"\n');
    
    // Simulate database query
    const allUsers = [
        {
            id: '1',
            name: 'John Doe',
            email: 'john@example.com',
            username: 'johndoe',
            active: true,
            createdAt: '2023-01-01T00:00:00Z'
        },
        {
            id: '2', 
            name: 'Jane Smith',
            email: 'jane@example.com',
            username: 'janesmith',
            active: true,
            createdAt: '2023-01-02T00:00:00Z'
        },
        {
            id: '3',
            name: 'Bob Johnson',
            email: 'bob@example.com', 
            username: 'bobjohnson',
            active: false,
            createdAt: '2023-01-03T00:00:00Z'
        }
    ];

    process.stderr.write('🔍 DEBUG: allUsers length: ' + allUsers.length + '\n');
    
    // Apply filter if provided
    let filteredUsers = allUsers;
    if (filter) {
        filteredUsers = allUsers.filter(user => 
            user.name.toLowerCase().includes(filter.toLowerCase()) ||
            user.email.toLowerCase().includes(filter.toLowerCase())
        );
        process.stderr.write('🔍 DEBUG: After filtering, users length: ' + filteredUsers.length + '\n');
    } else {
        process.stderr.write('🔍 DEBUG: No filter applied, keeping all users\n');
    }

    // Apply pagination
    const paginatedUsers = filteredUsers.slice(offset, offset + limit);
    process.stderr.write('🔍 DEBUG: After pagination (offset: ' + offset + ', limit: ' + limit + '), users length: ' + paginatedUsers.length + '\n');

    process.stderr.write('🎯 getUsersResolver returning: ' + JSON.stringify(paginatedUsers, null, 2) + '\n');
    return paginatedUsers;
}

async function getProductResolver(context, args) {
    process.stderr.write('🛍️ getProductResolver called with args: ' + JSON.stringify(args, null, 2) + '\n');
    process.stderr.write('🛍️ getProductResolver context: ' + JSON.stringify(context, null, 2) + '\n');
    
    const productId = getStringArg(args, 'productId', '1');
    
    // Simulate database lookup
    const product = {
        id: productId,
        name: 'Sample Product',
        description: 'This is a sample product from the JavaScript plugin',
        price: 29.99,
        stock: 100
    };

    process.stderr.write('🎯 getProductResolver returning: ' + JSON.stringify(product, null, 2) + '\n');
    return product;
}

async function getProductsPaginatedResolver(context, args) {
    process.stderr.write('📦 getProductsPaginatedResolver called with args: ' + JSON.stringify(args, null, 2) + '\n');
    process.stderr.write('📦 getProductsPaginatedResolver context: ' + JSON.stringify(context, null, 2) + '\n');
    
    const limit = getIntArg(args, 'limit', 10);
    const offset = getIntArg(args, 'offset', 0);
    const category = getStringArg(args, 'category', '');
    const minPrice = getFloatArg(args, 'minPrice', 0);
    const maxPrice = getFloatArg(args, 'maxPrice', 1000);
    
    // Simulate database query
    const allProducts = [
        {
            id: '1',
            name: 'Laptop Computer',
            description: 'High-performance laptop for professionals',
            price: 899.99,
            stock: 25
        },
        {
            id: '2',
            name: 'Wireless Mouse',
            description: 'Ergonomic wireless mouse with precision tracking',
            price: 29.99,
            stock: 100
        },
        {
            id: '3',
            name: 'Mechanical Keyboard',
            description: 'RGB mechanical keyboard for gaming and typing',
            price: 149.99,
            stock: 50
        }
    ];

    // Apply filters
    let filteredProducts = allProducts.filter(product => 
        product.price >= minPrice && product.price <= maxPrice
    );

    if (category) {
        filteredProducts = filteredProducts.filter(product =>
            product.name.toLowerCase().includes(category.toLowerCase())
        );
    }

    // Apply pagination
    const paginatedProducts = filteredProducts.slice(offset, offset + limit);

    const result = {
        products: paginatedProducts,
        total: filteredProducts.length,
        limit: limit,
        offset: offset,
        hasMore: offset + limit < filteredProducts.length
    };

    process.stderr.write('🎯 getProductsPaginatedResolver returning: ' + JSON.stringify(result, null, 2) + '\n');
    return result;
}

async function getTasksResolver(context, args) {
    process.stderr.write('📝 getTasksResolver called with args: ' + JSON.stringify(args, null, 2) + '\n');
    process.stderr.write('📝 getTasksResolver context: ' + JSON.stringify(context, null, 2) + '\n');
    
    const limit = getIntArg(args, 'limit', 10);
    const status = getStringArg(args, 'status', '');
    const completed = getBoolArg(args, 'completed', null);
    
    // Simulate database query
    const allTasks = [
        {
            id: '1',
            title: 'Implement user authentication',
            status: 'in_progress',
            completed: false,
            createdAt: '2023-01-01T00:00:00Z'
        },
        {
            id: '2',
            title: 'Design database schema',
            status: 'completed',
            completed: true,
            createdAt: '2023-01-02T00:00:00Z'
        },
        {
            id: '3',
            title: 'Write API documentation',
            status: 'pending',
            completed: false,
            createdAt: '2023-01-03T00:00:00Z'
        }
    ];

    // Apply filters
    let filteredTasks = allTasks;
    
    if (status) {
        filteredTasks = filteredTasks.filter(task => task.status === status);
    }
    
    if (completed !== null) {
        filteredTasks = filteredTasks.filter(task => task.completed === completed);
    }

    // Apply pagination
    const paginatedTasks = filteredTasks.slice(0, limit);

    const result = {
        tasks: paginatedTasks,
        total: filteredTasks.length,
        hasMore: filteredTasks.length > limit
    };

    process.stderr.write('🎯 getTasksResolver returning: ' + JSON.stringify(result, null, 2) + '\n');
        return result;
    }

async function createUserResolver(context, args) {
    process.stderr.write('➕ createUserResolver called with args: ' + JSON.stringify(args, null, 2) + '\n');
    
    const input = getObjectArg(args, 'input', {});
    
    const newUser = {
        id: Math.random().toString(36).substr(2, 9),
        name: input.name || 'Unknown',
        email: input.email || 'unknown@example.com',
        username: input.username || 'unknown',
        active: true,
        createdAt: new Date().toISOString()
    };

    const response = {
        message: 'User created successfully',
        success: true,
        data: newUser
    };

    return JSON.stringify(response, null, 2);
}

async function processBulkTagsResolver(context, args) {
    process.stderr.write('🏷️ processBulkTagsResolver called with args: ' + JSON.stringify(args, null, 2) + '\n');
    
    const userId = getStringArg(args, 'userId', '1');
    const tags = getArrayArg(args, 'tags', []);
    
    process.stderr.write(`Processing ${tags.length} tags for user ${userId}\n`);
    
    let processedCount = 0;
    let totalWeight = 0;
    
    for (const tag of tags) {
        if (tag.active) {
            processedCount++;
            totalWeight += tag.weight || 0;
        }
    }

    return `Processed ${processedCount} active tags for user ${userId}. Total weight: ${totalWeight}`;
}

// ========================================
// REST API HANDLER IMPLEMENTATIONS
// ========================================

async function helloRESTHandler(context, args) {
    process.stderr.write('🌐 helloRESTHandler called\n');
    logRESTArgs(args);

    return {
        message: 'Hello from REST API!',
        timestamp: new Date().toISOString(),
        method: 'GET',
        endpoint: '/hello'
    };
}

async function customHelloRESTHandler(context, args) {
    process.stderr.write('🌐 customHelloRESTHandler called\n');
    logRESTArgs(args);

    const name = getBodyParam(args, 'name', 'World');
    const message = getBodyParam(args, 'message', 'Hello');

    return {
        greeting: `${message}, ${name}!`,
        timestamp: new Date().toISOString(),
        method: 'POST',
        endpoint: '/custom-hello'
    };
}

async function getUserRESTHandler(context, args) {
    process.stderr.write('🔍 getUserRESTHandler called\n');
    logRESTArgs(args);

    const userId = getPathParam(args, 'id');
    const includeProfile = getQueryParam(args, 'include_profile', 'false') === 'true';
    const format = getQueryParam(args, 'format', 'json');

    const user = {
        id: userId,
        name: 'John Doe',
        email: 'john@example.com',
        active: true
    };

    if (includeProfile) {
        user.profile = {
            bio: 'Software developer',
            location: 'San Francisco, CA'
        };
    }

    return format === 'xml' ? 
        { xml: `<user><id>${user.id}</id><n>${user.name}</n></user>` } : 
        user;
}

async function createUserRESTHandler(context, args) {
    process.stderr.write('➕ createUserRESTHandler called\n');
    logRESTArgs(args);

    const name = getBodyParam(args, 'name', 'Unknown');
    const email = getBodyParam(args, 'email', 'unknown@example.com');
    const username = getBodyParam(args, 'username', 'unknown');
    const active = getBodyParam(args, 'active', true);

    const newUser = {
        id: Math.random().toString(36).substr(2, 9),
        name,
        email,
        username,
        active,
        createdAt: new Date().toISOString()
    };

    return {
        message: 'User created successfully',
        user: newUser,
        statusCode: 201
    };
}

async function updateUserRESTHandler(context, args) {
    process.stderr.write('✏️ updateUserRESTHandler called\n');
    logRESTArgs(args);

    const userId = getPathParam(args, 'id');
    const name = getBodyParam(args, 'name');
    const email = getBodyParam(args, 'email');
    const active = getBodyParam(args, 'active');

    const updatedUser = {
        id: userId,
        name: name || 'Updated Name',
        email: email || 'updated@example.com',
        active: active !== undefined ? active : true,
        updatedAt: new Date().toISOString()
    };

    return {
        message: 'User updated successfully',
        user: updatedUser,
        statusCode: 200
    };
}

async function deleteUserRESTHandler(context, args) {
    process.stderr.write('🗑️ deleteUserRESTHandler called\n');
    logRESTArgs(args);

    const userId = getPathParam(args, 'id');

    return {
        message: `User ${userId} deleted successfully`,
        statusCode: 200,
        deletedAt: new Date().toISOString()
    };
}

async function listUsersRESTHandler(context, args) {
    process.stderr.write('📋 listUsersRESTHandler called\n');
    logRESTArgs(args);

    const page = parseInt(getQueryParam(args, 'page', '1'));
    const pageSize = parseInt(getQueryParam(args, 'pageSize', '10'));
    const search = getQueryParam(args, 'search', '');
    const active = getQueryParam(args, 'active', null);

    // Simulate database
    let users = Array.from({ length: 50 }, (_, i) => ({
        id: `${i + 1}`,
        name: `User ${i + 1}`,
        email: `user${i + 1}@example.com`,
        active: i % 3 !== 0
    }));

    // Apply filters
    if (search) {
        users = users.filter(u => u.name.toLowerCase().includes(search.toLowerCase()));
    }
    
    if (active !== null) {
        const isActive = active === 'true';
        users = users.filter(u => u.active === isActive);
    }

    const total = users.length;
    const totalPages = Math.ceil(total / pageSize);
    const offset = (page - 1) * pageSize;
    const paginatedUsers = users.slice(offset, offset + pageSize);

    return {
        users: paginatedUsers,
        pagination: {
            page,
            pageSize,
            total,
            totalPages
        }
    };
}

async function uploadFileRESTHandler(context, args) {
    process.stderr.write('📤 uploadFileRESTHandler called\n');
    logRESTArgs(args);

    const filename = getBodyParam(args, 'filename', 'unknown.txt');
    const content = getBodyParam(args, 'content', '');
    const tags = getBodyParam(args, 'tags', []);
    const description = getBodyParam(args, 'description', '');

    const fileInfo = {
        id: Math.random().toString(36).substr(2, 9),
        filename,
        size: content.length,
        tags,
        description,
        uploadedAt: new Date().toISOString(),
        checksum: Math.random().toString(36).substr(2, 16)
    };

    return {
        message: 'File uploaded successfully',
        file: fileInfo,
        statusCode: 201
    };
}

async function statusRESTHandler(context, args) {
    process.stderr.write('📊 statusRESTHandler called\n');
    logRESTArgs(args);

    return {
        status: 'healthy',
        version: '2.0.3-sdk',
        uptime: process.uptime(),
        memory: process.memoryUsage(),
        timestamp: new Date().toISOString(),
        features: {
            graphqlQueries: 6,
            graphqlMutations: 2,
            restEndpoints: 10,
            customFunctions: 1
        }
    };
}

async function processComplexDataRESTHandler(context, args) {
    process.stderr.write('🔄 processComplexDataRESTHandler called\n');
    logRESTArgs(args);

    const user = getBodyParam(args, 'user', {});
    const preferences = getBodyParam(args, 'preferences', []);
    const metadata = getBodyParam(args, 'metadata', {});

    const processedData = {
        processId: Math.random().toString(36).substr(2, 9),
        user: {
            ...user,
            fullAddress: user.address ? 
                `${user.address.street}, ${user.address.city}, ${user.address.zip}` : 
                'No address provided'
        },
        preferences: {
            count: preferences.length,
            items: preferences
        },
        metadata: {
            keys: Object.keys(metadata),
            processed: true
        },
        processedAt: new Date().toISOString()
    };

    return {
        message: 'Complex data processed successfully',
        result: processedData,
        statusCode: 200
    };
}

// ========================================
// CUSTOM FUNCTION IMPLEMENTATIONS
// ========================================

async function customFunction(context, args) {
    process.stderr.write('⚙️ customFunction called with args: ' + JSON.stringify(args, null, 2) + '\n');

    return {
        message: 'Custom function executed successfully',
        timestamp: new Date().toISOString(),
        input: args,
        result: 'Function processing complete'
    };
}

// ========================================
// START THE PLUGIN
// ========================================

// Start the plugin
main().catch(error => {
    process.stderr.write('❌ Plugin failed to start: ' + error + '\n');
    process.exit(1);
}); 